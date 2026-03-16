package raftkv

import (
	"crypto/rand"
	"labrpc"
	"math/big"
	"time"
)

type Clerk struct {
	servers []*labrpc.ClientEnd
	clId    int
	seqNum  int
}

func nrand() int64 {
	max := big.NewInt(int64(1) << 62)
	bigx, _ := rand.Int(rand.Reader, max)
	x := bigx.Int64()
	return x
}

func MakeClerk(servers []*labrpc.ClientEnd) *Clerk {

	ck := new(Clerk)
	ck.servers = servers
	ck.clId = int(nrand())
	ck.seqNum = 0

	return ck
}

// fetch the current value for a key.
// returns "" if the key does not exist.
// keeps trying forever in the face of all other errors.

func (ck *Clerk) Get(key string) string {
	ck.seqNum++
	args := GetArgs{
		Key:    key,
		ClId:   ck.clId,
		SeqNum: ck.seqNum,
	}

	for {
		for i := range ck.servers {
			var reply GetReply
			ok := ck.servers[i].Call("RaftKV.Get", &args, &reply)
			if ok && !reply.WrongLeader {
				if reply.Err == OK {
					return reply.Value
				} else if reply.Err == ErrNoKey {
					return ""
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// shared by Put and Append.
//
// ok := ck.servers[i].Call("RaftKV.PutAppend", &args, &reply)
func (ck *Clerk) PutAppend(key string, value string, op string) {
	ck.seqNum++
	args := PutAppendArgs{
		Key:    key,
		Value:  value,
		ClId:   ck.clId,
		SeqNum: ck.seqNum,
		Op:     op,
	}

	for {
		for i := range ck.servers {
			var reply PutAppendReply
			ok := ck.servers[i].Call("RaftKV.PutAppend", &args, &reply)
			if ok && !reply.WrongLeader {
				if reply.Err == OK {
					return
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (ck *Clerk) Put(key string, value string) {
	ck.PutAppend(key, value, "Put")
}
func (ck *Clerk) Append(key string, value string) {
	ck.PutAppend(key, value, "Append")
}