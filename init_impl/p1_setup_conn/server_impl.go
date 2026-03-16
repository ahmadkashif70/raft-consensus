// Implementation of a KeyValueServer. Students should write their code in this file.

package pa1

import (
	"init_impl/rpcs"
	"bufio"
	"fmt"
	"net"
	"net/rpc"
	"net/http"
)

type keyValueServer struct {
	// TODO: implement this!

	clMap         map[net.Conn]chan string
	connClient    chan net.Conn
	disconnClient chan net.Conn
	getKv         chan string
	putKv         chan string
	clMapReq      chan string
	shutdown      chan bool
	countReq      chan int
	getRpc        chan *kvRpc
	putRpc		  chan *kvRpc

	clCount int
}

type kvRpc struct {
	key		string
	value 	[] byte
	reply	chan []byte
}

// New creates and returns (but does not start) a new KeyValueServer
func New() KeyValueServer {
	// TODO: implement this!
	return &keyValueServer{

		clMap:         make(map[net.Conn]chan string),
		connClient:    make(chan net.Conn),
		disconnClient: make(chan net.Conn),
		getKv:         make(chan string),
		putKv:         make(chan string),
		clMapReq:      make(chan string),
		shutdown:      make(chan bool),
		countReq:      make(chan int),
		getRpc: 	   make(chan *kvRpc),
		putRpc: 	   make(chan *kvRpc),

		clCount: 0,
	}
}

func (kvs *keyValueServer) StartModel1(port int) error {
	// TODO: implement this!

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))

	if err != nil {
		fmt.Printf("The connection could not be made to the port %d. Error: %s\n", port, err)
		return err
	}

	fmt.Printf("Listeining on port: %d", port)

	go kvs.handleMsgs()

	go func() {
		for {

			select {
			case <-kvs.shutdown:
				ln.Close()
				return

			default:
				conn, err := ln.Accept()

				if err != nil {
					fmt.Printf("The connection could not be made to the port %d. Error: %s\n", port, err)
				} else {
					kvs.connClient <- conn
				}
			}
		}
	}()

	return nil
}

func (kvs *keyValueServer) handleMsgs() {

	for {
		select {
		case conn := <-kvs.connClient:
			kvs.clCount += 1
			clChan := make(chan string, 500)
			kvs.clMap[conn] = clChan
			go kvs.handleClient(conn, clChan)

		case conn := <-kvs.disconnClient:
			kvs.clCount -= 1
			_, flag := kvs.clMap[conn]
			if flag {
				delete(kvs.clMap, conn)
			}
			conn.Close()

		case msg := <-kvs.getKv:

			parts := parseMsg(msg)
			if len(parts) == 2 {
				key := parts[1]
				val := get(key)
				reply := fmt.Sprintf("%s,%s\n", key, val)

				for _, channel := range kvs.clMap {
					select {
					case channel <- reply:
					default:
						continue
					}
				}
			} else {
				fmt.Printf("Invalid format. Correct format is: 'get,key'\n")
			}

		case msg := <-kvs.putKv:
			parts := parseMsg(msg)
			if len(parts) == 3 {
				key := parts[1]
				val := []byte(parts[2])
				put(key, val)
			} else {
				fmt.Printf("Invalid format. Correct format is: 'put,key,value'\n")
			}

		case kvs.countReq <- kvs.clCount:

		case <- kvs.shutdown:
			for conn := range kvs.clMap {
				conn.Close()
			}
			return
		}
	}
}

func (kvs *keyValueServer) handleClient(conn net.Conn, clChan chan string) {

	defer func() {
		select {
		case kvs.disconnClient <- conn:
		case <-kvs.shutdown:
		}
	}()

	rw := ConnectionToRW(conn)
	go kvs.sendReply(rw, clChan)

	for {
		msg, err := rw.ReadString('\n')
		if err != nil {
			break
		}

		if len(msg) >= 3 {
			switch {
			case msg[:3] == "put":
				select {
				case kvs.putKv <- msg:
				case <-kvs.shutdown:
					return
				}
			case msg[:3] == "get":
				select {
				case kvs.getKv <- msg:
				case <-kvs.shutdown:
					return
				}
			}
		}
	}
}

func (kvs *keyValueServer) sendReply(rw *bufio.ReadWriter, clChan chan string) {

	for msg := range clChan {
		_, err := rw.WriteString(msg)
		if err != nil {
			fmt.Printf("Error writing to client \n")
			return
		}

		err = rw.Flush()
		if err != nil {
			fmt.Printf("Error writing to client after flush \n")
			return
		}
	}
}

func (kvs *keyValueServer) Close() {
	// TODO: implement this!

	close(kvs.shutdown)
}

func (kvs *keyValueServer) Count() int {
	// TODO: implement this!
	return <-kvs.countReq
}

func parseMsg(msg string) []string {
	var parts []string
	var currChar []byte

	for i := 0; i < len(msg); i++ {
		if msg[i] == ',' {
			parts = append(parts, string(currChar))
			currChar = []byte{}
		} else if msg[i] != '\n' {
			currChar = append(currChar, msg[i])
		}
	}

	if len(currChar) != 0 {
		parts = append(parts, string(currChar))
	}

	return parts
}

func ConnectionToRW(conn net.Conn) *bufio.ReadWriter {
	return bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
}

func (kvs *keyValueServer) StartModel2(port int) error {
	// TODO: implement this!
	//
	// Do not forget to call rpcs.Wrap(...) on your kvs struct before
	// passing it to <sv>.Register(...)
	//
	// Wrap ensures that only the desired methods (RecvGet and RecvPut)
	// are available for RPC access. Other KeyValueServer functions
	// such as Close(), StartModel1(), etc. are forbidden for RPCs.
	//
	// Example: <sv>.Register(rpcs.Wrap(kvs))

	sv := rpc.NewServer()
	err := sv.Register(rpcs.Wrap(kvs))

	if err != nil {
		return fmt.Errorf("failed to rgeister RPC server : %s", err)
	}

	http.DefaultServeMux = http.NewServeMux()
	sv.HandleHTTP(rpc.DefaultRPCPath, rpc.DefaultDebugPath)

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %v", port, err)
	}

	go kvs.handleRpcMsg()

	go func() {
		for {
			err := http.Serve(ln, nil)
			if err != nil {
				select {
                case <-kvs.shutdown:
                    return
                default:
                    continue
                }
			}
		}
	}()

	return nil
}

func (kvs *keyValueServer) RecvGet(args *rpcs.GetArgs, reply *rpcs.GetReply) error {
	// TODO: implement this!

	resultChan := make(chan []byte)
	kvs.getRpc <- &kvRpc {
		key:   args.Key,
		reply: resultChan,
	}

	val := <-resultChan
	if val == nil {
		fmt.Println("Key does not exist in Data Base Server")
	}
	reply.Value = val

	return nil
}

func (kvs *keyValueServer) RecvPut(args *rpcs.PutArgs, reply *rpcs.PutReply) error {
	// TODO: implement this!
	resultChan := make(chan []byte)
	kvs.putRpc <- &kvRpc {
		key:   args.Key,
		value: args.Value,
		reply: resultChan,
	}

	<-resultChan
	
	return nil
}

// TODO: add additional methods/functions below!

func (kvs *keyValueServer) handleRpcMsg() {
	for {
		select {
		case req := <-kvs.getRpc:

			val := get(req.key);

			if val != nil {
				req.reply <- val
			} else {
				req.reply <- nil
			}

		case req := <-kvs.putRpc:

			put(req.key, req.value)
			req.reply <- nil

		case <-kvs.shutdown:
			return
		}
	}
}


