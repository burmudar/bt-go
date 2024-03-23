package peer

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bt"
	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bt/types"
	"golang.org/x/sync/semaphore"
)

type MessageHandler func(msg Message) error

const MaxSendMessages = 1

type ChannelState int

var Choked ChannelState = 1
var Unchoked ChannelState = 2
var ErrorState ChannelState = 3
var Closed ChannelState = 4

func (s ChannelState) String() string {
	switch s {
	case Choked:
		return "Choked"
	case Unchoked:
		return "Unchoked"
	case ErrorState:
		return "ErrorState"
	case Closed:
		return "Closed"

	}
	return "Unknown"
}

var ErrPieceUnavailable error = fmt.Errorf("peer does not have requested piece")

var DEBUG = false

type Channel struct {
	ConnectedTo string
	Handshake   *Handshake
	BitField    *BitField

	sync.Mutex
	chokeCond     *sync.Cond
	sendSemaphore semaphore.Weighted

	conn  net.Conn
	state *atomic.Value

	send chan Message
	Done chan struct{}

	onRecvHooks map[MessageTag]MessageHandler

	Err error
}

func NewHandshakedChannel(ctx context.Context, peerID string, p *types.Peer, torrent *types.Torrent) (*Channel, error) {
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", p.String())
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	h, err := doHandshake(ctx, conn, peerID, torrent.Hash)
	fieldSize := bt.Ceil(torrent.GetPieceCount(), 8)
	bitField := &BitField{Field: make([]byte, fieldSize)}
	return NewChannel(conn, h, bitField), err
}

func NewChannel(conn net.Conn, handshake *Handshake, bitField *BitField) *Channel {
	var state atomic.Value
	state.Store(Unchoked)
	ch := &Channel{
		Mutex:       sync.Mutex{},
		ConnectedTo: conn.RemoteAddr().String(),
		Handshake:   handshake,
		BitField:    bitField,
		conn:        conn,
		send:        make(chan Message, MaxSendMessages),
		Done:        make(chan struct{}),

		chokeCond: sync.NewCond(&sync.Mutex{}),
		state:     &state,

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
	if !ch.IsValid() {
		return fmt.Errorf("[%s] in invalid state", ch.ConnectedTo)
	}
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
	ch.debug("<- %s", msg.String())
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

func (ch *Channel) GetState() ChannelState {
	v := ch.state.Load().(ChannelState)
	return v
}

func (ch *Channel) SetState(v ChannelState) {
	ch.state.Store(v)
}

func (ch *Channel) IsState(v ...ChannelState) bool {
	current := ch.GetState()
	for _, other := range v {
		if current == other {
			return true
		}
	}

	return false
}

func (ch *Channel) IsChoked() bool {
	return ch.IsState(Choked) && ch.IsValid()
}

func (ch *Channel) IsValid() bool {
	return !ch.IsState(ErrorState, Closed)
}

func (ch *Channel) writer() {
	buf := bufio.NewWriter(ch.conn)
	defer ch.Close()
	defer ch.debug("<<< writer exiting >>>")

	ctx := context.Background()
	for {
		select {
		case <-ch.Done:
			return
		case m := <-ch.send:
			{
				if m == nil {
					ch.log("got nil message - ignoring")
					continue
				}
				ch.chokeCond.L.Lock()
				if ch.IsChoked() {
					ch.log("choked - waiting")
					ch.chokeCond.Wait()
				}
				ch.chokeCond.L.Unlock()

				ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
				err := WriteMessage(ctx, buf, m)
				cancel()
				buf.Flush()
				ch.debug("-> %s", m.String())
				if err != nil {
					ch.setError(err)
					if err == ctx.Err() {
						return
					}
					ch.log("failed to write message: %v\n", err)
				}

			}
		}
	}
}

func (ch *Channel) reader() {
	buf := bufio.NewReader(ch.conn)
	defer ch.Close()

	defer ch.debug("<<< reader exiting >>>")

	ctx := context.Background()
	for {
		select {
		case <-ch.Done:
			return
		default:
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			msg, err := DecodeMessage(ctx, buf)
			cancel()
			if err != nil {
				ch.setError(err)
				if err == ctx.Err() {
					ch.debug("context Deadline exceeded - returning")
					return
				} else if err == io.EOF {
					ch.debug("EOF - returning")
					return
				} else {
					ch.log("failed to decode raw message: %v", err)
					continue
				}

			}

			ch.handleMessage(msg)
		}
	}

}

func (ch *Channel) setError(err error) {
	ch.Lock()
	defer ch.Unlock()
	ch.SetState(ErrorState)
	ch.Err = err
}

func (ch *Channel) debug(format string, args ...any) {
	if DEBUG {
		prefix := fmt.Sprintf("[%s]", ch.ConnectedTo)
		value := fmt.Sprintf(format, args...)
		fmt.Printf("%s %s\n", prefix, value)
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
	ch.Lock()
	defer ch.Unlock()
	if !ch.IsState(Closed) {
		ch.SetState(Closed)
		close(ch.Done)
		close(ch.send)
		ch.debug("closed")
	} else {
		ch.debug("already Closed")
	}
}

func (ch *Channel) handleChoke(msg Message) error {
	ch.chokeCond.L.Lock()
	ch.SetState(Choked)
	defer ch.chokeCond.L.Unlock()
	ch.fireReceiveHook(msg)
	return nil
}
func (ch *Channel) handleUnchoke(msg Message) error {
	ch.chokeCond.L.Lock()
	defer ch.chokeCond.L.Unlock()
	if ch.IsChoked() {
		ch.SetState(Unchoked)
		ch.chokeCond.Signal()
	}
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
	offset := idx % 8
	ch.Lock()
	defer ch.Unlock()
	return ch.BitField.Field[byteIdx]>>(7-offset)&1 != 0
}

func (ch *Channel) SetPiece(idx int) {
	byteIdx := idx / 8
	offset := idx % 8
	ch.Lock()
	defer ch.Unlock()
	ch.BitField.Field[byteIdx] |= 1 << (7 - offset)
	ch.log("setting piece %d in bitfield", idx)
}
