// package raftkv

// import (
// 	"encoding/gob"
// 	"labrpc"
// 	"log"
// 	"raft"
// 	"sync"
// )

// const Debug = 0

// func DPrintf(format string, a ...interface{}) (n int, err error) {
// 	if Debug > 0 {
// 		log.Printf(format, a...)
// 	}
// 	return
// }

// // Define the operation structure
// type Op struct {
// 	Key      string
// 	Value    string
// 	Type     string
// 	ClientID int64
// 	SeqNum   int
// }

// // RaftKV server struct
// type RaftKV struct {
// 	mu           sync.Mutex
// 	me           int
// 	rf           *raft.Raft
// 	applyCh      chan raft.ApplyMsg
// 	maxraftstate int

// 	database          map[string]string //kv-store
// 	requestChannels   map[int]chan Op   //every request has a unique channel
// 	processedRequests map[int64]int     //latest seen request from client <client, requestID>
// }

// func (kv *RaftKV) Get(args *GetArgs, reply *GetReply) {
// 	op := kv.createOp(args.Key, "", "Get", args.ClientID, args.RequestID)
// 	if kv.isDuplicateRequest(args.ClientID, args.RequestID) { //check for duplicates
// 		kv.respondToGet(args.Key, reply)
// 		return
// 	}
// 	kv.processOperation(op, reply) //otherwise call start
// }

// func (kv *RaftKV) PutAppend(args *PutAppendArgs, reply *PutAppendReply) {
// 	op := kv.createOp(args.Key, args.Value, args.Op, args.ClientID, args.RequestID)
// 	if kv.isDuplicateRequest(args.ClientID, args.RequestID) {
// 		reply.WrongLeader = false
// 		reply.Err = OK
// 		return
// 	}
// 	kv.processOperation(op, reply)
// }

// func (kv *RaftKV) createOp(key, value, opType string, clientID int64, seqNum int) Op {
// 	return Op{
// 		Key:      key,
// 		Value:    value,
// 		Type:     opType,
// 		ClientID: clientID,
// 		SeqNum:   seqNum,
// 	}
// }

// func (kv *RaftKV) isDuplicateRequest(clientID int64, seqNum int) bool {
// 	kv.mu.Lock()
// 	defer kv.mu.Unlock()
// 	lastSeq, exists := kv.processedRequests[clientID] //return true if duplicate
// 	return exists && seqNum <= lastSeq
// }

// func (kv *RaftKV) respondToGet(key string, reply *GetReply) {
// 	kv.mu.Lock()
// 	defer kv.mu.Unlock()
// 	reply.Value = kv.database[key] //retrive value
// 	reply.WrongLeader = false
// 	reply.Err = OK
// }

// func (kv *RaftKV) processOperation(op Op, reply interface{}) {
// 	index, _, isLeader := kv.rf.Start(op)
// 	if !isLeader {
// 		setWrongLeader(reply) //call start and see if current node is leader
// 		return
// 	}

// 	ch := kv.getRequestChan(index)

// 	select {
// 	case result := <-ch:
// 		if kv.isMatchingOperation(op, result) { //verify if same request
// 			kv.setReplySuccess(result, reply)
// 		} else {
// 			setWrongLeader(reply)
// 		}
// 	}
// }

// func (kv *RaftKV) getRequestChan(index int) chan Op {
// 	kv.mu.Lock()
// 	defer kv.mu.Unlock()
// 	if _, exists := kv.requestChannels[index]; !exists {
// 		kv.requestChannels[index] = make(chan Op, 1) //return channel for this command
// 	}
// 	return kv.requestChannels[index]
// }

// func (kv *RaftKV) isMatchingOperation(op, result Op) bool {
// 	return op.ClientID == result.ClientID && op.SeqNum == result.SeqNum
// }

// func (kv *RaftKV) setReplySuccess(op Op, reply interface{}) {
// 	switch r := reply.(type) {
// 	case *GetReply:
// 		r.Value = op.Value
// 		r.WrongLeader = false
// 		r.Err = OK
// 	case *PutAppendReply:
// 		r.WrongLeader = false
// 		r.Err = OK
// 	}
// }

// func (kv *RaftKV) applyOperation(op Op) string {

// 	if op.Type == "Put" { //write value in put
// 		kv.database[op.Key] = op.Value
// 	} else if op.Type == "Append" { //concatenate if exists or write if new
// 		kv.database[op.Key] += op.Value
// 	}
// 	return kv.database[op.Key]
// }

// func (kv *RaftKV) listener() {
// 	for msg := range kv.applyCh { //committed command returned
// 		kv.mu.Lock()
// 		op := msg.Command.(Op)
// 		value := kv.applyOperation(op)
// 		kv.updateprocessedRequests(op)
// 		kv.notifyWaitChannel(msg.Index, op, value)
// 		kv.mu.Unlock()
// 	}
// }

// func (kv *RaftKV) updateprocessedRequests(op Op) {
// 	if lastSeq, ok := kv.processedRequests[op.ClientID]; !ok || op.SeqNum > lastSeq {
// 		kv.processedRequests[op.ClientID] = op.SeqNum
// 	}
// }

// func (kv *RaftKV) notifyWaitChannel(index int, op Op, value string) {
// 	if replyChannel, exists := kv.requestChannels[index]; exists {
// 		op.Value = value
// 		replyChannel <- op
// 	}
// }

// func (kv *RaftKV) Kill() {
// 	kv.rf.Kill()
// }

// func StartKVServer(servers []*labrpc.ClientEnd, me int, persister *raft.Persister, maxraftstate int) *RaftKV {
// 	gob.Register(Op{})

// 	kv := &RaftKV{
// 		me:           me,
// 		maxraftstate: maxraftstate,
// 		applyCh:      make(chan raft.ApplyMsg),
// 	}

// 	kv.rf = raft.Make(servers, me, persister, kv.applyCh)

// 	kv.initialiseServer()

// 	go kv.listener()
// 	return kv
// }

// func (kv *RaftKV) initialiseServer() {
// 	kv.database = make(map[string]string)
// 	kv.requestChannels = make(map[int]chan Op)
// 	kv.processedRequests = make(map[int64]int)
// 	return
// }

// func setWrongLeader(reply interface{}) {
// 	switch r := reply.(type) {
// 	case *GetReply:
// 		r.WrongLeader = true
// 	case *PutAppendReply:
// 		r.WrongLeader = true
// 	}
// }