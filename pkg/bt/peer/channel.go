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
var ErrorState ChannelState = 3

var ErrPieceUnavailable error = fmt.Errorf("peer does not have requested piece")

type Channel struct {
	ConnectedTo string
	Handshake   *Handshake
	BitField    *BitField

	sync.Mutex
	chokeCond *sync.Cond

	conn  net.Conn
	state ChannelState

	send chan Message
	done chan struct{}

	onRecvHooks map[MessageTag]MessageHandler

	Err error
}

func NewHandshakedChannel(ctx context.Context, peerID string, p *types.Peer, hash [20]byte) (*Channel, error) {
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", p.String())
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	h, err := doHandshake(ctx, conn, peerID, hash)
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

func (ch *Channel) WaitFor(ctx context.Context, tag MessageTag) error {
	recv := make(chan Message)

	ch.RegisterReceiveHook(tag, func(_ Message) error {
		close(recv)
		return nil
	})

	defer ch.RemoveReceiveHook(tag)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-recv:
			return nil
		}
	}
}

func (ch *Channel) handleMessage(msg Message) error {
	ch.log("handling %s", msg.String())
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
			ch.log("no handler for message: %s", m.String())
		}
	}

	return nil
}

func (ch *Channel) IsChoked() bool {
	return ch.state == Choked && ch.IsValid()
}

func (ch *Channel) IsValid() bool {
	return ch.state != ErrorState
}

func (ch *Channel) writer() {
	buf := bufio.NewWriter(ch.conn)

	for m := range ch.send {
		ch.chokeCond.L.Lock()
		if ch.IsChoked() {
			ch.log("choked - waiting")
			ch.chokeCond.Wait()
		}
		ch.chokeCond.L.Unlock()
		err := WriteMessage(buf, m)
		buf.Flush()
		ch.log("sent %s", m.String())
		if err != nil {
			ch.log("failed to write message: %v\n", err)
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
			ch.log("reading from buffer")
			msg, err := DecodeMessage(buf)
			if err != nil {
				if err == io.EOF {
					ch.log("reader exit - %v", err)
					ch.Lock()
					ch.state = ErrorState
					ch.Err = io.EOF
					ch.Unlock()
					ch.Close()
					return
				}
				ch.log("failed to decode raw message: %v", err)
				continue
			}

			ch.log("handling message %T", msg)
			ch.handleMessage(msg)
		}
	}

}

func (ch *Channel) log(format string, args ...any) {
	prefix := fmt.Sprintf("[%s]", ch.ConnectedTo)
	value := fmt.Sprintf(format, args...)
	fmt.Printf("%s %s\n", prefix, value)
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
	ch.fireReceiveHook(msg)
	return nil
}
func (ch *Channel) handleNotInterested(msg Message) error {
	ch.fireReceiveHook(msg)
	return nil
}
func (ch *Channel) handleHave(msg Message) error {
	ch.fireReceiveHook(msg)
	return nil
}

func (ch *Channel) handleBitField(msg Message) error {
	v, ok := msg.(*BitField)
	if !ok {
		return fmt.Errorf("expected BitField got %T", msg)
	}
	ch.Lock()
	ch.BitField = v
	defer ch.Unlock()
	ch.fireReceiveHook(msg)
	return nil
}
func (ch *Channel) handlePieceBlock(blk *PieceBlock) error {
	ch.fireReceiveHook(blk)
	return nil
}
func (ch *Channel) handlePieceRequest(msg Message) error {
	ch.fireReceiveHook(msg)
	return nil
}
func (ch *Channel) handleCancel(msg Message) error {
	ch.fireReceiveHook(msg)
	return nil
}
func (ch *Channel) handleKeepAlive(msg Message) error {
	ch.fireReceiveHook(msg)
	return nil
}

func (ch *Channel) HasPiece(idx int) bool {
	byteIdx := idx / 8
	offset := byteIdx % 8
	ch.Lock()
	defer ch.Unlock()
	return ch.BitField.Field[byteIdx]>>(7-offset)&1 != 0
}

func (ch *Channel) SetPiece(idx int) {
	byteIdx := idx / 8
	offset := byteIdx % 8
	ch.Lock()
	defer ch.Unlock()
	ch.log("setting piece %d in bitfield", idx)
	ch.BitField.Field[byteIdx] |= 1 << (7 - offset)
}
