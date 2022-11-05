package main

import (
	"gitee.com/geektime-geekbang/geektime-go/micro/v5/rpc"
	"gitee.com/geektime-geekbang/geektime-go/micro/v5/rpc/serialize/json"
	"gitee.com/geektime-geekbang/geektime-go/micro/v5/rpc/serialize/proto"
)

func main() {
	svr := rpc.NewServer()
	svr.RegisterService(&UserService{})
	svr.RegisterService(&UserServiceProto{})
	svr.RegisterSerializer(json.Serializer{})
	svr.RegisterSerializer(proto.Serializer{})
	if err := svr.Start(":8081"); err != nil {
		panic(err)
	}
}
