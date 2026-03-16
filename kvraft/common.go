package raftkv

const (
	OK       = "OK"
	ErrNoKey = "ErrNoKey"
)

type Err string

type PutAppendArgs struct {
	Key   string
	Value string
	Op    string
	ClId int
	SeqNum int
}

type PutAppendReply struct {
	WrongLeader bool
	Err         Err
}

type GetArgs struct {
	Key string
	SeqNum int
	ClId int
}

type GetReply struct {
	WrongLeader bool
	Err         Err
	Value       string
}
