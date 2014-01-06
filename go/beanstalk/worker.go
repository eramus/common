package beanstalk

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/kr/beanstalk"
)

type Request struct {
	RequestId uint64      `json:"request"`
	Action    string      `json:"action"`
	Data      interface{} `json:"data"`
}

type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
}

type WorkFunc func(int, *Request) (Response, string, error)

func Run(workerId int, workerFunc WorkFunc, deadWorker chan<- bool) {

	beanConn, err := beanstalk.Dial("tcp", "0.0.0.0:11300")
	if err != nil {
		fmt.Println("BEANSTALK:", err)
		return
	}
	defer func() {
		r := recover()
		if r != nil {
			fmt.Println("PANIC:", r)
			deadWorker <- true
		}
		beanConn.Close()
	}()

	var req Request

	for {
		id, msg, err := beanConn.Reserve(10 * time.Second)
		if err != nil {
			cerr, ok := err.(beanstalk.ConnError)
			if ok && cerr.Err == beanstalk.ErrTimeout {
				continue
			} else {
				fmt.Println("SOMETHING BAD HAPPENED TO BEANSTALK")
				panic("conn err")
			}
		}
		fmt.Printf("[%d:%d] START JOB\n", workerId, time.Now().UnixNano())

		err = json.Unmarshal(msg, &req)
		if err != nil {
			fmt.Println("JSON:", err)
			panic("json err")
		}

		fmt.Printf("[%d:%d] DATA: %v\n", workerId, time.Now().UnixNano(), req)

		response, tube, err := workerFunc(workerId, &req)
		if err != nil {
			fmt.Println("WORK:", err)
			panic("work err")
		}

		jsonRes, err := json.Marshal(response)
		if err != nil {
			fmt.Println("JSON:", err)
			panic("json err")
		}
		fmt.Printf("[%d:%d] %s\n", workerId, time.Now().UnixNano(), string(jsonRes))

		beanConn.Tube.Name = tube
		_, err = beanConn.Put(jsonRes, 0, 0, (3600 * time.Second))
		if err != nil {
			fmt.Println("BEANSTALK WRITE:", err)
			panic("write err")
		}

		beanConn.Delete(id)

		fmt.Printf("[%d:%d] FINISHED REQUEST\n", workerId, time.Now().UnixNano())
	}
}
