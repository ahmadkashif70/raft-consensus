package raftkv

import (
	"encoding/gob"
	"labrpc"
	"log"
	"raft"
	"sync"
	"time"
)

const Debug = 0

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug > 0 {
		log.Printf(format, a...)
	}
	return
}

type Op struct {
    OpType string
    Key    string
    Value  string
    Id     int
    SeqNum int
}


type RaftKV struct {
	mu      sync.Mutex
	me      int
	rf      *raft.Raft
	applyCh chan raft.ApplyMsg

	maxraftstate int // snapshot if log grows this big
	kvs          map[string]string
	clSeq        map[int]int
	notifyCh     map[int]chan Op
}

func (kv *RaftKV) Get(args *GetArgs, reply *GetReply) {
	// Your code here.
	op := Op{
		OpType: "Get",
		Key:    args.Key,
		Id:     args.ClId,
		SeqNum: args.SeqNum,
	}

	index, _, isLeader := kv.rf.Start(op)
	if !isLeader {
		reply.WrongLeader = true
		return
	}

	// DPrintf("Get: %v, index: %v, isLeader: %v\n", args.Key, index, isLeader)
	kv.mu.Lock()
	ch := kv.getNotifyCh(index)
	kv.mu.Unlock()

	select {
	case committedOp := <-ch:
		if committedOp.Id == op.Id && committedOp.SeqNum == op.SeqNum {
			kv.mu.Lock()
			value, exists := kv.kvs[op.Key]
			if exists {
				reply.Value = value
				reply.Err = OK
			} else {
				reply.Err = ErrNoKey
			}
			kv.mu.Unlock()
		} else {
			reply.WrongLeader = true
		}
	case <-time.After(250 * time.Millisecond):
		reply.WrongLeader = true
	}

	kv.mu.Lock()
	delete(kv.notifyCh, index)
	kv.mu.Unlock()

}

func (kv *RaftKV) PutAppend(args *PutAppendArgs, reply *PutAppendReply) {
	op := Op{
		OpType: args.Op,
		Key:    args.Key,
		Value:  args.Value,
		Id:     args.ClId,
		SeqNum: args.SeqNum,
	}

	index, _, isLeader := kv.rf.Start(op)
	if !isLeader {
		reply.WrongLeader = true
		return
	}

	kv.mu.Lock()
	ch := kv.getNotifyCh(index)
	kv.mu.Unlock()

	select {
	case committedOp := <-ch:
		if committedOp.Id == op.Id && committedOp.SeqNum == op.SeqNum {
			reply.Err = OK
		} else {
			reply.WrongLeader = true
		}
	case <-time.After(250 * time.Millisecond):
		reply.WrongLeader = true
	}

	kv.mu.Lock()
	delete(kv.notifyCh, index)
	kv.mu.Unlock()
}

func (kv *RaftKV) getNotifyCh(index int) chan Op {
	_, ok := kv.notifyCh[index]
	if !ok {
		kv.notifyCh[index] = make(chan Op)
	}
	return kv.notifyCh[index]
}

// the tester calls Kill() when a RaftKV instance won't
// be needed again. you are not required to do anything
// in Kill(), but it might be convenient to (for example)
// turn off debug output from this instance.
func (kv *RaftKV) Kill() {
	kv.rf.Kill()
	// Your code here, if desired.
}

// servers[] contains the ports of the set of
// servers that will cooperate via Raft to
// form the fault-tolerant key/value service.
// me is the index of the current server in servers[].
// the k/v server should store snapshots with persister.SaveSnapshot(),
// and Raft should save its state (including log) with persister.SaveRaftState().
// the k/v server should snapshot when Raft's saved state exceeds maxraftstate bytes,
// in order to allow Raft to garbage-collect its log. if maxraftstate is -1,
// you don't need to snapshot.
// StartKVServer() must return quickly, so it should start goroutines
// for any long-running work.
func StartKVServer(servers []*labrpc.ClientEnd, me int, persister *raft.Persister, maxraftstate int) *RaftKV {
    gob.Register(Op{})

    kv := new(RaftKV)
    kv.me = me
    kv.maxraftstate = maxraftstate

    kv.applyCh = make(chan raft.ApplyMsg)
    kv.rf = raft.Make(servers, me, persister, kv.applyCh)

    kv.kvs = make(map[string]string)
    kv.clSeq = make(map[int]int)
    kv.notifyCh = make(map[int]chan Op)

    go kv.applyChListener()

    return kv
}

func (kv *RaftKV) applyChListener() {
    for msg := range kv.applyCh {

        op, ok := msg.Command.(Op)
        if !ok {
            continue
        }

        kv.mu.Lock()
        lastSeq, seen := kv.clSeq[op.Id]
        if !seen || op.SeqNum > lastSeq {
            switch op.OpType {
            case "Put":
                kv.kvs[op.Key] = op.Value
            case "Append":
                kv.kvs[op.Key] += op.Value
            }
            kv.clSeq[op.Id] = op.SeqNum
        }

        if ch, exists := kv.notifyCh[msg.Index]; exists {
            ch <- op
        }
        kv.mu.Unlock()
    }
}