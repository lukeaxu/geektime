package orm

import (
	"context"
	"gitee.com/geektime-geekbang/geektime-go/demo/internal/errs"
)

// Selector 用于构造 SELECT 语句
type Selector[T any] struct {
	builder
	core
	table string
	where []Predicate
	// db *DB
	sess Session
	columns []Selectable
}

type Selectable interface {
	// selectable(b *builder) error
	selectable()
}

// s.Select("id", "age")
func (s *Selector[T]) Select(cols...Selectable) *Selector[T] {
	s.columns = cols
	return s
}

func (s *Selector[T]) Use(ms...Middleware) *Selector[T] {
	s.ms = ms
	return s
}

// 万一我的 T 是基础类型
func (s *Selector[T]) Get(ctx context.Context) (*T, error) {
	var root Handler = func(ctx context.Context, qc *QueryContext) *QueryResult {
		q, err := qc.Builder.Build()
		if err != nil {
			return &QueryResult{
				Err: err,
			}
		}

		rows, err := s.sess.queryContext(ctx, q.SQL, q.Args...)
		if err != nil {
			return &QueryResult{
				Err: err,
			}
		}

		t := new(T)
		val := s.valCreator(t, s.model)
		// 在这里灵活切换反射或者 unsafe
		err = val.SetColumns(rows)
		return &QueryResult{
			Result: t,
			Err: err,
		}
	}
	for i := len(s.ms) - 1; i >= 0 ; i-- {
		root = s.ms[i](root)
	}

	m := s.model
	if m == nil {
		var err error
		m, err = s.r.Get(new(T))
		if err != nil {
			return nil, err
		}
	}
	res := root(ctx, &QueryContext{
		Type: "SELECT",
		Model: m,
		Builder: s,
		TableName: s.table,
		DBName: s.dbName,
	})

	if res.Result != nil {
		return res.Result.(*T), res.Err
	}
	return nil, res.Err
	// if res.Err != nil {
	// 	return nil, res.Err
	// }
	// t, ok := res.Result.(*T)
	// if ok {
	// 	return t, nil
	// }
	// return nil, errors.New("orm: 非正确类型")
}

func (s *Selector[T]) GetMulti(ctx context.Context) ([]*T, error) {
	// var db *sql.DB
	// q, err := s.Build()
	// if err != nil {
	// 	return nil, err
	// }
	// rows, err := db.QueryContext(ctx, q.SQL, q.Args...)
	// if err != nil {
	// 	return nil, err
	// }
	// 想办法，把 rows 所有行转换为 []*T
	panic("implement me")
}

// From 指定表名，如果是空字符串，那么将会使用默认表名
func (s *Selector[T]) From(tbl string) *Selector[T] {
	s.table = tbl
	return s
}

func (s *Selector[T]) Build() (*Query, error) {
	t := new(T)
	var err error
	s.model, err = s.r.Get(t)
	if err != nil {
		return nil, err
	}
	s.sb.WriteString("SELECT ")
	if len(s.columns) == 0 {
		s.sb.WriteByte('*')
	} else {
		for i, c := range s.columns {
			if i > 0 {
				s.sb.WriteByte(',')
			}
			switch col := c.(type) {
			case Column:
				fd, ok := s.model.FieldMap[col.name]
				if !ok {
					return nil, errs.NewErrUnknownField(col.name)
				}
				s.sb.WriteByte('`')
				s.sb.WriteString(fd.ColName)
				s.sb.WriteByte('`')
			case Aggregate:
				s.sb.WriteString(col.fn)
				s.sb.WriteByte('(')
				fd, ok := s.model.FieldMap[col.arg]
				if !ok {
					return nil, errs.NewErrUnknownField(col.arg)
				}
				s.sb.WriteByte('`')
				s.sb.WriteString(fd.ColName)
				s.sb.WriteByte('`')
				s.sb.WriteByte(')')
			case RawExpr:
				s.sb.WriteString(col.raw)
				if len(col.args) >0 {
					s.args = append(s.args, col.args...)
				}
			}
		}
	}
	s.sb.WriteString(" FROM ")
	if s.table == "" {
		s.sb.WriteByte('`')
		s.sb.WriteString(s.model.TableName)
		s.sb.WriteByte('`')
	} else {
		s.sb.WriteString(s.table)
	}

	// 构造 WHERE
	if len(s.where) > 0 {
		// 类似这种可有可无的部分，都要在前面加一个空格
		s.sb.WriteString(" WHERE ")
		p := s.where[0]
		for i := 1; i < len(s.where); i++ {
			p = p.And(s.where[i])
		}
		if err := s.buildExpression(p); err != nil {
			return nil, err
		}
	}
	s.sb.WriteString(";")
	return &Query{
		SQL: s.sb.String(),
		Args: s.args,
	}, nil
}

func (s *Selector[T]) buildExpression(e Expression) error {
	if e == nil {
		return nil
	}
	switch exp := e.(type) {
	case Column:
		s.sb.WriteByte('`')
		fd, ok := s.model.FieldMap[exp.name]
		if !ok {
			return errs.NewErrUnknownField(exp.name)
		}
		s.sb.WriteString(fd.ColName)
		s.sb.WriteByte('`')
	case value:
		s.sb.WriteByte('?')
		s.args = append(s.args, exp.val)
	case Predicate:
		_, lp := exp.left.(Predicate)
		if lp {
			s.sb.WriteByte('(')
		}
		if err := s.buildExpression(exp.left); err != nil {
			return err
		}
		if lp {
			s.sb.WriteByte(')')
		}

		s.sb.WriteByte(' ')
		s.sb.WriteString(exp.op.String())
		s.sb.WriteByte(' ')

		_, rp := exp.right.(Predicate)
		if rp {
			s.sb.WriteByte('(')
		}
		if err := s.buildExpression(exp.right); err != nil {
			return err
		}
		if rp {
			s.sb.WriteByte(')')
		}
	default:
		return errs.NewErrUnsupportedExpressionType(exp)
	}
	return nil
}

// Where 用于构造 WHERE 查询条件。如果 ps 长度为 0，那么不会构造 WHERE 部分
func (s *Selector[T]) Where(ps ...Predicate) *Selector[T] {
	s.where = ps
	return s
}

// cols 是用于 WHERE 的列，难以解决 And Or 和 Not 等问题
// func (s *Selector[T]) Where(cols []string, args...any) *Selector[T] {
// 	s.whereCols = cols
// 	s.args = append(s.args, args...)
// }

// 最为灵活的设计
// func (s *Selector[T]) Where(where string, args...any) *Selector[T] {
// 	s.where = where
// 	s.args = append(s.args, args...)
// }

// 可以同时用在 DB 和 Tx 上，我就需要为它们提供一个统一的抽象
func NewSelector[T any](sess Session) *Selector[T] {
	return &Selector[T]{
		sess: sess,
		core: sess.getCore(),
	}
}

// func NewSelector[T any](tx *sql.Tx) *Selector[T] {
//
// }
