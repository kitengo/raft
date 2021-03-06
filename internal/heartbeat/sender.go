package heartbeat

import (
	raftmember "github.com/kitengo/raft/internal/member"
	raftmodels "github.com/kitengo/raft/internal/models"
	raftsender "github.com/kitengo/raft/internal/sender"
	raftstate "github.com/kitengo/raft/internal/state"
	raftterm "github.com/kitengo/raft/internal/term"
	"log"
	"sync"
)

type Payload struct {
	MemberID     string
	Term         uint64
	LeaderID     string
	LeaderCommit uint64
}

type RaftHeartbeat interface {
	SendHeartbeats()
}

func NewRaftHeartbeat(raftTerm raftterm.RaftTerm,
	raftMember raftmember.RaftMember,
	raftIndex *raftstate.RaftIndex,
	raftSender raftsender.RequestSender) RaftHeartbeat {
	return &raftHeartbeat{
		raftMember: raftMember,
		raftTerm:   raftTerm,
		raftIndex:  raftIndex,
		raftSender: raftSender,
	}
}

type raftHeartbeat struct {
	raftMember raftmember.RaftMember
	raftTerm   raftterm.RaftTerm
	raftIndex  *raftstate.RaftIndex
	raftSender raftsender.RequestSender
}

func (rhb *raftHeartbeat) SendHeartbeats() {
	term := rhb.raftTerm.GetTerm()
	leader := rhb.raftMember.Leader()
	leaderCommit := rhb.raftIndex.GetCommitOffset()
	log.Println("Sending heartbeat from leader...")
	//Get a list of all the members
	members, err := rhb.raftMember.List()
	if err != nil {
		log.Printf("unable to list members of the cluster %v\n", err)
	}

	heartbeat := Payload{
		Term:         term,
		LeaderID:     leader.ID,
		LeaderCommit: leaderCommit,
	}

	//Initialize the waitgroup
	var wg sync.WaitGroup
	for _, member := range members {
		wg.Add(1)
		go rhb.sendHeartbeat(&wg, member, &heartbeat)
	}
	log.Printf("Done sending heartbeats to all the members")
	wg.Wait()
}

func (rhb *raftHeartbeat) sendHeartbeat(wg *sync.WaitGroup, member raftmember.Entry, payload *Payload) {
	defer wg.Done()
	aePayload := raftmodels.AppendEntryPayload{
		Term:         payload.Term,
		LeaderId:     payload.LeaderID,
		LeaderCommit: payload.LeaderCommit,
	}
	resp, err := rhb.raftSender.SendCommand(&aePayload, member.Address, member.Port)
	if err != nil {
		//Requeue the entry back into the buffer channel
		log.Printf("Unable to send AppendEntry request %v\n", err)
		return
	}
	var aeResp raftmodels.AppendEntryResponse
	err = aeResp.FromPayload(resp.Payload)
	if err != nil {
		log.Printf("Unable to decode AppendEntry response")
		return
	}
}
