package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import (
	// "fmt"
	"labrpc"
	"math/rand"
	"sync"
	"time"
    "bytes"
    "encoding/gob"
)

// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make().
type ApplyMsg struct {
	Index       int
	Command     interface{}
	UseSnapshot bool   // ignore for Assignment2; only used in Assignment3
	Snapshot    []byte // ignore for Assignment2; only used in Assignment3
}

type LogEntry struct {
	Term int
	Command interface{}
}

// A Go object implementing a single Raft peer.
type Raft struct {
	mu        sync.Mutex
	peers     []*labrpc.ClientEnd
	persister *Persister
	me        int // index into peers[]

	currentTerm     int
	votedFor        int
	isLeader        bool
	state           string
	voteCount       int
	electionTimeout time.Duration
	recvChan        chan bool

	nextIndex       []int
    matchIndex      []int
	log				[]LogEntry
	commitIndex		int
	lastApplied		int
	applyChan		chan ApplyMsg
	// Your data here.
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	var term int
	var isleader bool
	// Your code here.
	rf.mu.Lock()
	defer rf.mu.Unlock()
	term = rf.currentTerm
	if rf.state == "leader" {
		isleader = true
	} else {
		isleader = false
	}

	rf.isLeader = isleader

	return term, isleader
}

// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
func (rf *Raft) persist() {
	
    w := new(bytes.Buffer)
    e := gob.NewEncoder(w)
    e.Encode(rf.currentTerm)
    e.Encode(rf.votedFor)
    e.Encode(rf.log)

    data := w.Bytes()
    rf.persister.SaveRaftState(data)
}

// restore previously persisted state.
func (rf *Raft) readPersist(data []byte) {
	
    r := bytes.NewBuffer(data)
    d := gob.NewDecoder(r)
    d.Decode(&rf.currentTerm)
    d.Decode(&rf.votedFor)
    d.Decode(&rf.log)
}

// example RequestVote RPC arguments structure.
type RequestVoteArgs struct {
	
	TermNum     int
	CandidateId int
	LastLogIndex int
	LastLogTerm int
}

// example RequestVote RPC reply structure.
type RequestVoteReply struct {
	
	TermNum     int
	VoteGranted bool
}

// example RequestVote RPC handler.
func (rf *Raft) RequestVote(args RequestVoteArgs, reply *RequestVoteReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	// fmt.Println("in reqVote handler")

	if args.TermNum < rf.currentTerm {
		reply.TermNum = rf.currentTerm
		reply.VoteGranted = false
		return
	}

	if args.TermNum > rf.currentTerm {
		rf.currentTerm = args.TermNum
		rf.state = "follower"
		rf.votedFor = -1
        rf.persist()
	}

	reply.TermNum = rf.currentTerm

	
	lastLogIndex := len(rf.log) - 1
    lastLogTerm := 0
    if lastLogIndex >= 0 {
        lastLogTerm = rf.log[lastLogIndex].Term
    }

	// fmt.Println("checking Logindex and term")
	upToDate := false
    if args.LastLogTerm > lastLogTerm {
        upToDate = true
		// fmt.Println("UPDATED con 1")
    } else if args.LastLogTerm == lastLogTerm && args.LastLogIndex >= lastLogIndex {
        upToDate = true
		// fmt.Println("UPDATED con 2")
    }

	if (rf.votedFor == -1 || rf.votedFor == args.CandidateId) && upToDate {
        rf.votedFor = args.CandidateId
        reply.VoteGranted = true
        rf.persist()
		// fmt.Printf("VOTE GRANTED!! to %d \n", args.CandidateId)

        select {
        case rf.recvChan <- true:
        default:
        }

    } else {
        reply.VoteGranted = false
    }
}

// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// returns true if labrpc says the RPC was delivered.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
func (rf *Raft) sendRequestVote(server int, args RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}

type AppendEntriesArgs struct {
	Term     		int
	LeaderId 		int
	PrevLogIndex 	int
	PrevLogTerm 	int
	Entries 		[]LogEntry
	LeaderCommit 	int
}

type AppendEntriesReply struct {
	Term int
	Success bool
}

func (rf *Raft) AppendEntries(args AppendEntriesArgs, reply *AppendEntriesReply) {
    rf.mu.Lock()
    defer rf.mu.Unlock()

    if args.Term < rf.currentTerm {
        reply.Term = rf.currentTerm
        reply.Success = false
        return
    }

    if args.Term > rf.currentTerm {
        rf.currentTerm = args.Term
        rf.state = "follower"
        rf.votedFor = -1
        rf.persist()
    }

    rf.state = "follower"

    // Reset election timeout
    select {
    case rf.recvChan <- true:
    default:
    }

    reply.Term = rf.currentTerm

    if args.PrevLogIndex >= len(rf.log) || rf.log[args.PrevLogIndex].Term != args.PrevLogTerm {
        reply.Success = false
        return
    }

    rf.log = rf.log[:args.PrevLogIndex+1]

    for i := 0; i < len(args.Entries); i++ {
        rf.log = append(rf.log, args.Entries[i])
    }

    rf.persist()

    if args.LeaderCommit > rf.commitIndex {
        rf.commitIndex = min(args.LeaderCommit, len(rf.log)-1)
        go rf.applyEntries()
    }

    reply.Success = true
}


func (rf *Raft) getRandomElectionTimeout() time.Duration {
	return time.Duration(400+rand.Intn(200)) * time.Millisecond
}

func (rf *Raft) sendAppendEntries(server int, args AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}

func (rf *Raft) updateCommitIndex() {
    for i := len(rf.log) - 1; i > rf.commitIndex; i-- {
        count := 1
        for j := range rf.peers {
            if j != rf.me && rf.matchIndex[j] >= i && rf.log[i].Term == rf.currentTerm {
                count++
            }
        }
        if count > len(rf.peers)/2 {
            rf.commitIndex = i
            go rf.applyEntries()
            break
        }
    }
}


// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
func (rf *Raft) Start(command interface{}) (int, int, bool) {
    rf.mu.Lock()
    defer rf.mu.Unlock()

    index := 0
    term := rf.currentTerm
    isLeader := (rf.state == "leader")

    if !isLeader {
        return index, term, isLeader
    }

    index = len(rf.log)
    rf.log = append(rf.log, LogEntry{Term: term, Command: command})
    rf.persist()

    rf.matchIndex[rf.me] = index
    rf.nextIndex[rf.me] = index + 1

    // fmt.Printf("[Leader %d] Appended new entry at index %d\n", rf.me, index)

    // go rf.sendHeartbeats()

    return index, term, isLeader
}


// the tester calls Kill() when a Raft instance won't
// be needed again. you are not required to do anything
// in Kill(), but it might be convenient to (for example)
// turn off debug output from this instance.
func (rf *Raft) Kill() {
	// Your code here, if desired.
}


func (rf *Raft) beginElection() {
	rf.mu.Lock()
	if rf.state == "leader" {
		rf.mu.Unlock()
        return
    }
	rf.currentTerm += 1
	rf.state = "candidate"
	rf.votedFor = rf.me
	rf.voteCount = 1
    rf.persist()
	
	term := rf.currentTerm
	me := rf.me
	peers := rf.peers
	rf.electionTimeout = rf.getRandomElectionTimeout()

	lastLogIndex := len(rf.log) - 1
    lastLogTerm := 0
    if lastLogIndex >= 0 {
        lastLogTerm = rf.log[lastLogIndex].Term
    }

    rf.mu.Unlock()

	// fmt.Println("starting election")

	for i := range peers {
		if i != me {
			go func(server int) {
				args := RequestVoteArgs{
					TermNum:     term,
					CandidateId: me,
					LastLogIndex: lastLogIndex,
					LastLogTerm: lastLogTerm,
				}
				reply := &RequestVoteReply{}

				ok := rf.sendRequestVote(server, args, reply)
				if ok {
					rf.mu.Lock()
					defer rf.mu.Unlock()

					if rf.state != "candidate" || rf.currentTerm != term {
						return
					}

					if reply.TermNum > rf.currentTerm {
						rf.currentTerm = reply.TermNum
						rf.state = "follower"
						rf.votedFor = -1
                        rf.persist()
						return
					}
					 
					if reply.VoteGranted {
						rf.voteCount += 1
						if rf.voteCount > len(rf.peers)/2 && rf.state != "leader" {
							// fmt.Printf("Leader Selected! New Leader is %d \n", me)
							rf.state = "leader"
							rf.isLeader = true
							go rf.sendHeartbeats()
						}
					}
				}
			}(i)
		}
	}
}

func (rf *Raft) sendHeartbeats() {
    for {
        rf.mu.Lock()
        if rf.state != "leader" {
            rf.mu.Unlock()
            return
        }

        currentTerm := rf.currentTerm
        leaderCommit := rf.commitIndex
        me := rf.me

        rf.mu.Unlock()

        for i := range rf.peers {
            if i != me {
                go func(server int) {
                    rf.mu.Lock()
                    if rf.state != "leader" {
                        rf.mu.Unlock()
                        return
                    }

                    prevLogIndex := rf.nextIndex[server] - 1
                    prevLogTerm := rf.log[prevLogIndex].Term
                    entries := make([]LogEntry, len(rf.log[rf.nextIndex[server]:]))
                    copy(entries, rf.log[rf.nextIndex[server]:])

                    args := AppendEntriesArgs{
                        Term:         currentTerm,
                        LeaderId:     me,
                        PrevLogIndex: prevLogIndex,
                        PrevLogTerm:  prevLogTerm,
                        Entries:      entries,
                        LeaderCommit: leaderCommit,
                    }
                    rf.mu.Unlock()

                    reply := &AppendEntriesReply{}
                    ok := rf.sendAppendEntries(server, args, reply)
                    if ok {
                        rf.mu.Lock()
                        defer rf.mu.Unlock()

                        if reply.Term > rf.currentTerm {
                            rf.currentTerm = reply.Term
                            rf.state = "follower"
                            rf.votedFor = -1
                            rf.isLeader = false
                            rf.persist()
                        } else if reply.Success {
                            rf.matchIndex[server] = args.PrevLogIndex + len(args.Entries)
                            rf.nextIndex[server] = rf.matchIndex[server] + 1
                            rf.updateCommitIndex()
                        } else {
                            rf.nextIndex[server] = max(1, rf.nextIndex[server]-1)
                        }
                    }
                }(i)
            }
        }
        time.Sleep(100 * time.Millisecond)
    }
}




func (rf *Raft) heartbeatListener() {
    for {
        rf.mu.Lock()
        timeout := rf.electionTimeout
        rf.mu.Unlock()
		timer := time.After(timeout)

		select {
		case <-rf.recvChan:
			rf.mu.Lock()
			rf.electionTimeout = rf.getRandomElectionTimeout()
			rf.mu.Unlock()
			continue
		case <-timer:
			rf.mu.Lock()
			if rf.state != "leader" {
				rf.mu.Unlock()
				// rf.state = "candidate"
				rf.beginElection()
			} else {
				rf.mu.Unlock()
			}
		}
    }
}

func (rf *Raft) applyEntries() {
    rf.mu.Lock()
    defer rf.mu.Unlock()

    for rf.lastApplied < rf.commitIndex && rf.lastApplied+1 < len(rf.log) { // new condition daalni ha k nhi 
        rf.lastApplied++
        entry := rf.log[rf.lastApplied]

        applyMsg := ApplyMsg{
            Index:   rf.lastApplied,
            Command: entry.Command,
        }
        rf.applyChan <- applyMsg
    }
}

// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me
	rf.applyChan = applyCh

	// Your initialization code here.
	rf.currentTerm = 0
	rf.votedFor = -1
	rf.isLeader = false
	rf.state = "follower"
	rf.voteCount = 0
	rf.electionTimeout = rf.getRandomElectionTimeout()
	rf.recvChan = make(chan bool)

	rf.log = make([]LogEntry, 1)
    rf.log[0] = LogEntry{Term: 0}
    rf.nextIndex = make([]int, len(rf.peers))
    rf.matchIndex = make([]int, len(rf.peers))
    for i := range rf.peers {
        rf.nextIndex[i] = 1
        rf.matchIndex[i] = 0
    }

	rf.readPersist(persister.ReadRaftState())
	
	go rf.heartbeatListener()
	go rf.applyEntries()

	return rf
}
