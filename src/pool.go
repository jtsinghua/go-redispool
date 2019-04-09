package src

import (
	"fmt"
	"github.com/go-redis/redis"
	"github.com/labstack/gommon/log"
	"time"
)

//redis连接池
type RedisPool struct {
	//创建连接的方法
	DialFunc func() *redis.Client
	//在归还的时候，测试连接是否可用
	TestOnBorrow func(client *redis.Client) error
	//核心连接数
	CoreSize uint16
	//最大空闲连接数
	MaxIdle uint16
	//最大连接数
	MaxSize uint16
	//最大空闲连接等待时间，过了等待时间，即关闭
	IdleTimeout time.Duration
	//若是连接池中已经没有连接，是否等待
	//若是不等待，则直接报错
	WaitBlocking bool
	//若是连接池耗尽，超时等待
	WaitTimeout time.Duration
}

//获取连接方法
//从头部取，还是从尾部取
func BorrowCli() (*redis.Client, error) {
	if !isUsable {
		log.Fatalf("BorrowCli: 请先调用Init函数初始化")
		return nil, nil
	}
	//连接池中没有连接
	//且已经超过了最大连接数，则不能再创建，只能等待
	if idleCli == 0 && (RPool.MaxSize+RPool.MaxIdle) <= totalCli {
		//连接池耗尽，也不能再新创建连接，若是配置了阻塞等待
		//阻塞等待
		if RPool.WaitBlocking {
			//若是超过一个协程在等待
			//等超时，则所有协程会一起获取，则有些必然无法获取
			//然后再查看连接池中是否已经有连接
			time.Sleep(RPool.WaitTimeout)
			if idleCli != 0 {
				cli := <-clients
				if usingCli > 0 {
					usingCli--
				}
				idleCli++
				return cli, nil
			} else {
				return nil, TimeoutError{
					msg: "连接池耗尽，等待超时",
				}
			}
		} else {
			return nil, QueueEmptyError{
				msg: "连接池耗尽，没有连接可用",
			}
		}
	} else if idleCli == 0 && RPool.MaxSize+RPool.MaxIdle > totalCli {
		client := RPool.DialFunc()
		//创建了新连接
		totalCli++
		usingCli++
		return client, nil
	} else {
		cli := <-clients
		if cli == nil {
			return nil, QueueEmptyError{
				msg: "无法获取连接",
			}
		}

		usingCli++

		if idleCli > 0 {
			idleCli--
		}
		//测试连接是否有效
		err := RPool.TestOnBorrow(cli)
		if err != nil {
			//说明连接不可用
			totalCli--
			log.Fatalf("BorrowCli: 连接无效，请重新获取: %s\n", err.Error())
			return nil, err
		}

		return cli, nil
	}
}

//归还连接方法
//如果归还失败，需要手动关闭
func ReturnCli(cli *redis.Client) {
	//测试连接的可用性
	err := RPool.TestOnBorrow(cli)
	if err != nil {
		if usingCli > 0 {
			usingCli--
		}

		if totalCli > 0 {
			totalCli--
		}
		log.Fatalf("ReturnCli: 连接无效")
		return
	}
	//入队列
	clients <- cli
	//空闲连接数增加
	idleCli++
	usingCli--
}

//全局变量
var RPool *RedisPool

//保存连接
//用通道模拟消费者和生产者
var clients chan *redis.Client

//创建的总连接数
var totalCli uint16 = 0

//正在使用的连接数
var usingCli uint16 = 0

//空闲连接数
var idleCli uint16 = 0

//标志位，连接池是否可用
var isUsable = false

//获取RedisPool
func Init() {
	//初始化
	RPool = &RedisPool{
		CoreSize:     8,
		MaxIdle:      3,
		MaxSize:      15,
		IdleTimeout:  time.Second * 60,
		WaitBlocking: true,
		//等待1秒
		WaitTimeout: time.Second,
		DialFunc: func() *redis.Client {
			cli := redis.NewClient(&redis.Options{
				Addr:     "localhost:6379",
				Password: "",
				DB:       0,
			})

			return cli
		},
		TestOnBorrow: func(client *redis.Client) error {
			_, err := client.Ping().Result()
			return err
		},
	}
	//初始化队列
	//clients = NewQueue(16)
	//带缓存通道
	//最大225个连接
	clients = make(chan *redis.Client, 255)
	//初始化核心连接数个连接
	fmt.Println("初始化核心数量的连接")
	for i := uint16(0); i < RPool.CoreSize; i++ {
		cli := RPool.DialFunc()
		err := RPool.TestOnBorrow(cli)
		if err != nil {
			continue
		}

		clients <- cli
	}

	isUsable = true
}
