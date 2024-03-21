package peer

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bt/types"
)

type MessageHandler func(msg Message) error

const MaxSendMessages = 1

type ChannelState int

var Choked ChannelState = 1
var Unchoked ChannelState = 2
var Invalid ChannelState = 3

type Channel struct {
	ConnectedTo string
	Handshake   *Handshake

	sync.Mutex
	chokeCond *sync.Cond

	conn  net.Conn
	state ChannelState

	send chan Message
	done chan struct{}

	onRecvHooks map[MessageTag]MessageHandler

	pieces []*PieceBlock
}

func NewHandshakedChannel(ctx context.Context, peerID string, p *types.Peer, hash [20]byte) (*Channel, error) {
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", p.String())
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	println("doing handshake", p.String())
	h, err := doHandshake(ctx, conn, peerID, hash)
	println("handshake done", p.String())
	return NewChannel(conn, h), err
}

func NewChannel(conn net.Conn, handshake *Handshake) *Channel {
	ch := &Channel{
		Mutex:       sync.Mutex{},
		ConnectedTo: conn.RemoteAddr().String(),
		Handshake:   handshake,
		conn:        conn,
		send:        make(chan Message, MaxSendMessages),
		done:        make(chan struct{}),

		chokeCond: sync.NewCond(&sync.Mutex{}),
		state:     Unchoked,

		onRecvHooks: map[MessageTag]MessageHandler{},
	}

	go ch.reader()
	go ch.writer()

	return ch
}

func (ch *Channel) SendUnchoke() error {
	if !ch.IsValid() {
		return fmt.Errorf("[%s] in invalid state", ch.ConnectedTo)
	}
	ch.send <- &Unchoke{}
	return nil
}

func (ch *Channel) SendInterested() error {
	if !ch.IsValid() {
		return fmt.Errorf("[%s] in invalid state", ch.ConnectedTo)
	}
	ch.send <- &Interested{}
	return nil
}

func (ch *Channel) SendPieceRequest(index, begin, length int) error {
	if !ch.IsValid() {
		return fmt.Errorf("[%s] in invalid state", ch.ConnectedTo)
	}
	ch.send <- &PieceRequest{
		Index:  index,
		Begin:  begin,
		Length: length,
	}

	return nil
}

func (ch *Channel) SendHave(index int) error {
	ch.send <- &Have{index}
	return nil
}

func (ch *Channel) WaitFor(tag MessageTag) error {
	recv := make(chan Message)

	ch.RegisterReceiveHook(tag, func(_ Message) error {
		close(recv)
		return nil
	})

	<-recv
	return nil
}

func (ch *Channel) handleMessage(msg Message) error {
	switch m := msg.(type) {
	case *Choke:
		{
			return ch.handleChoke(m)
		}
	case *Unchoke:
		{
			return ch.handleUnchoke(m)
		}
	case *Interested:
		{
			return ch.handleInterested(m)
		}
	case *NotInterested:
		{
			return ch.handleNotInterested(m)
		}
	case *Have:
		{
			return ch.handleHave(m)
		}
	case *BitField:
		{
			return ch.handleBitField(m)
		}
	case *PieceRequest:
		{
			return ch.handlePieceRequest(m)
		}
	case *PieceBlock:
		{
			return ch.handlePieceBlock(m)
		}
	case *Cancel:
		{
			return ch.handleCancel(m)
		}
	case *KeepAlive:
		{
			return ch.handleKeepAlive(m)
		}
	default:
		{
			fmt.Printf("no handler for message: %s\n", m.String())
		}
	}

	return nil
}

func (ch *Channel) IsChoked() bool {
	return ch.state == Choked && ch.IsValid()
}

func (ch *Channel) IsValid() bool {
	return ch.state != Invalid
}

func (ch *Channel) writer() {
	buf := bufio.NewWriter(ch.conn)

	for m := range ch.send {
		ch.chokeCond.L.Lock()
		if ch.IsChoked() {
			fmt.Printf("[%s] choked - waiting\n", ch.ConnectedTo)
			ch.chokeCond.Wait()
		}
		ch.chokeCond.L.Unlock()
		fmt.Printf("[%s] sending %s\n", ch.ConnectedTo, m.String())
		err := WriteMessage(buf, m)
		fmt.Printf("[%s] flushing %s\n", ch.ConnectedTo, m.String())
		buf.Flush()
		if err != nil {
			fmt.Printf("failed to write message: %v\n", err)
		}
	}
}

func (ch *Channel) reader() {
	buf := bufio.NewReader(ch.conn)

	for {
		select {
		case <-ch.done:
			return
		default:
			fmt.Printf("[%s] reading from buffer\n", ch.ConnectedTo)
			msg, err := DecodeMessage(buf)
			if err != nil {
				if err == io.EOF {
					fmt.Printf("reader exit - %v\n", err)
					ch.Lock()
					ch.state = Invalid
					ch.Unlock()
					ch.Close()
					return
				}
				fmt.Printf("failed to decode raw message: %v", err)
				continue
			}

			fmt.Printf("[%s] handling message %T\n", ch.ConnectedTo, msg)
			ch.handleMessage(msg)
		}
	}

}

func (ch *Channel) RegisterReceiveHook(tag MessageTag, h MessageHandler) {
	ch.Lock()
	ch.onRecvHooks[tag] = h
	defer ch.Unlock()
}

func (ch *Channel) RemoveReceiveHook(tag MessageTag) {
	ch.Lock()
	delete(ch.onRecvHooks, tag)
	defer ch.Unlock()
}

func (ch *Channel) Close() {
	close(ch.send)
	close(ch.done)
}

func (ch *Channel) handleChoke(msg Message) error {
	ch.Lock()
	ch.state = Choked
	ch.chokeCond.L.Lock()
	defer ch.Unlock()
	ch.fireReceiveHook(msg)
	return nil
}
func (ch *Channel) handleUnchoke(msg Message) error {
	ch.chokeCond.L.Lock()
	ch.state = Unchoked
	ch.chokeCond.Signal()
	ch.fireReceiveHook(msg)
	return nil
}

func (ch *Channel) fireReceiveHook(msg Message) {
	fn, ok := ch.onRecvHooks[msg.Tag()]
	if ok {
		fn(msg)
	}
}

func (ch *Channel) handleInterested(msg Message) error {
	fmt.Printf("[%s] received %s\n", ch.ConnectedTo, msg.String())
	ch.fireReceiveHook(msg)
	return nil
}
func (ch *Channel) handleNotInterested(msg Message) error {
	fmt.Printf("[%s] received %s\n", ch.ConnectedTo, msg.String())
	ch.fireReceiveHook(msg)
	return nil
}
func (ch *Channel) handleHave(msg Message) error {
	fmt.Printf("[%s] received %s\n", ch.ConnectedTo, msg.String())
	ch.fireReceiveHook(msg)
	return nil
}
func (ch *Channel) handleBitField(msg Message) error {
	fmt.Printf("[%s] received %s\n", ch.ConnectedTo, msg.String())
	ch.fireReceiveHook(msg)
	return nil
}
func (ch *Channel) handlePieceBlock(blk *PieceBlock) error {
	fmt.Printf("[%s] received %s\n", ch.ConnectedTo, blk.String())
	ch.pieces = append(ch.pieces, blk)
	ch.fireReceiveHook(blk)
	return nil
}
func (ch *Channel) handlePieceRequest(msg Message) error {
	fmt.Printf("[%s] received %s\n", ch.ConnectedTo, msg.String())

	ch.fireReceiveHook(msg)
	return nil
}
func (ch *Channel) handleCancel(msg Message) error {
	fmt.Printf("[%s] received %s\n", ch.ConnectedTo, msg.String())
	ch.fireReceiveHook(msg)
	return nil
}
func (ch *Channel) handleKeepAlive(msg Message) error {
	fmt.Printf("[%s] received %s\n", ch.ConnectedTo, msg.String())
	ch.fireReceiveHook(msg)
	return nil
}
