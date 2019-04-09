# go-redispool

###使用方法
在需要使用连接池的文件的init函数中调用Init函数

````
func init() {
	Init()
}

````
````
//获取消息id
func GetMessageId() uint64 {
	cli, err := BorrowCli()

	if err != nil {
		log.Fatalf("BorrowCli: %s\n", err.Error())
		return 0
	}

	defer ReturnCli(cli)

	id, err := cli.Get("newMessageId").Uint64()
	if err != nil {
		log.Printf("cli.Get: %s\n", err.Error())
		return 0
	}
	//增加，以备下一个获取
	cli.Incr("newMessageId")

	return id
}
``