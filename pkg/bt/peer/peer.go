package peer

import (
	"bufio"
	"context"
	"crypto/sha1"
	"fmt"
	"net"
	"os"
	"sort"
	"time"

	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bt/types"
)

const (
	BitTorrentProtocol = "BitTorrent protocol"
	HandshakeLength    = 1 + 19 + 20 + 20 // length + protocol string + hash + peerid

)

type Channel struct {
	conn net.Conn
	send chan Message
	done chan struct{}
}

func NewChannel(conn net.Conn) *Channel {
	ch := &Channel{
		conn: conn,
		send: make(chan Message),
		done: make(chan struct{}),
	}

	go ch.reader()
	go ch.writer()

	return ch
}

func (ch *Channel) reader() {
	buf := bufio.NewReader(ch.conn)

	for {
		select {
		case <-ch.done:
			return
		default:
			msg, err := DecodeMessage(buf)
			if err != nil {
				fmt.Println("<--- failed to decode raw message ---->")
				continue
			}

			ch.handleMessage(msg)
		}
	}
}

func (ch *Channel) handleMessage(msg Message) {
	switch m := msg.(type) {
	case ChokeType:
		{
			return ch.handleChoke(m)
		}
	case UnchokeType:
		{
			return ch.handleUnchoke(m)
		}
	case InterestedType:
		{
			return ch.handleInterested(m)
		}
	case NotInterestedType:
		{
			return ch.handleNotInterested(m)
		}
	case HaveType:
		{
			return ch.handleMessage(m)
		}
	case BitFieldType:
		{
			return ch.handleBitField(m)
		}
	case RequestType:
		{
			return ch.handleRequest(m)
		}
	case PieceType:
		{
			return ch.handlePiece(m)
		}
	case CancelType:
		{
			return ch.handleCancel(m)
		}
	case KeepAliveType:
		{
			return ch.handleCancel(m)
		}
	}
}

func (ch *Channel) writer() {
	buf := bufio.NewWriter(ch.conn)

	for m := range ch.send {
		err := WriteMessage(buf, m)
		if err != nil {
			fmt.Println("<---- failed to write message ---->")
		}
	}
}

func (ch *Channel) Close() {
	close(ch.send)
	close(ch.done)
}

type Client struct {
	debug         bool
	PeerID        string
	Peer          *types.Peer
	conn          net.Conn
	bufrw         *bufio.ReadWriter
	lastHandshake *Handshake

	choked bool
}

type Result[T any] struct {
	R   T
	Err error
}

func (c *Client) writeMessage(msg Message) error {
	data := EncodeMessage(msg)
	return c.send(data)
}

func NewClient(peerID string) *Client {
	return &Client{
		debug:  os.Getenv("DEBUG") == "1",
		PeerID: peerID,
	}
}

func NewHandshakedClient(ctx context.Context, id string, peer *types.Peer, torrent *types.Torrent) (*Client, error) {
	client := NewClient(id)

	if err := client.Connect(ctx, peer); err != nil {
		return nil, fmt.Errorf("failed to connect to client: %v", err)
	}

	if _, err := client.Handshake(ctx, torrent.Hash); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to perform handshake to client: %v", err)
	}
	return client, nil
}

func (c *Client) Handshaked() bool {
	return c.lastHandshake != nil
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *Client) Connect(ctx context.Context, p *types.Peer) error {
	c.announcef("connecting to: %s\n", p.String())
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", p.String())
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	c.Peer = p
	c.conn = conn
	c.bufrw = bufio.NewReadWriter(bufio.NewReader(c.conn), bufio.NewWriter(c.conn))

	c.announcef("connected to: %s\n", p.String())

	return nil
}

func (c *Client) IsConnected() bool {
	return c.conn != nil
}

func (c *Client) send(data []byte) error {
	c.announcef("sending %d\n", len(data))
	if _, err := c.conn.Write(data); err != nil {
		return fmt.Errorf("send failure: %w", err)
	}
	return nil
}

func resultWithContext[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	done := make(chan Result[T])
	go func() {
		r, err := fn()
		done <- Result[T]{
			R:   r,
			Err: err,
		}
	}()
	select {
	case <-ctx.Done():
		{
			var empty T
			return empty, ctx.Err()
		}
	case result := <-done:
		{
			return result.R, result.Err
		}
	}
}

func (c *Client) Handshake(ctx context.Context, hash [20]byte) (*Handshake, error) {
	doHandshake := func() (*Handshake, error) {
		if !c.IsConnected() {
			return nil, fmt.Errorf("not connected")
		}

		c.announcef("starting handshake ...\n")

		data, err := encodeHandshake(&Handshake{
			PeerID: c.PeerID,
			Hash:   hash,
		})
		if err != nil {
			return nil, fmt.Errorf("encoding failure: %w", err)
		}
		if err := c.send(data); err != nil {
			return nil, err
		}

		resp := [68]byte{}
		if _, err := read(c.bufrw, resp[:]); err != nil {
			return nil, err
		}

		h, err := decodeHandshake(resp[:])
		if err == nil {
			c.lastHandshake = h
		}
		c.announcef("handshake complete...\n")

		return h, err
	}

	return resultWithContext[*Handshake](ctx, doHandshake)
}

func (c *Client) waitForUnchoke() error {
	ticker := time.NewTicker(2 * time.Second)
	done := time.NewTimer(60 * time.Second)

	for {
		select {
		case <-ticker.C:
			{
				c.announcef("<unchoke loop> sending \"interested\"\n")
				msg, err := c.Interested()
				if err != nil {
					c.announcef("<unchoke loop> failed to send interested")
					continue
				}

				if msg.Tag() == UnchokeType {
					c.choked = false
					c.announcef("<unchoke loop> received unchoke - %T\n", msg)
					ticker.Stop()
					done.Stop()
					return nil
				} else {
					c.announcef("<unchoke loop> waiting for unchoke - got %T\n", msg)
				}
			}
		case <-done.C:
			{
				ticker.Stop()
				done.Stop()
				return fmt.Errorf("failed to unchoke after 30 seconds")
			}
		}
	}
}

func assembleData(blocks []*PieceBlock) ([]byte, error) {
	var sortErr error
	sort.Slice(blocks, func(i, j int) bool {
		if blocks[i] == nil {
			sortErr = fmt.Errorf("blocks at i: %d were nil", i)
			return false
		}
		if blocks[j] == nil {
			sortErr = fmt.Errorf("blocks at j: %d were nil", j)
			return false
		}
		return blocks[i].Begin < blocks[j].Begin
	})

	if sortErr != nil {
		return nil, sortErr
	}

	// The last block may be smaller than the regular chunk size
	data := make([]byte, 0)
	for _, block := range blocks {
		data = append(data, block.Data...)
	}

	return data, nil
}

func (c *Client) BitField() (Message, error) {
	c.announcef("read bitfield message...\n")
	msg, err := DecodeMessage(c.bufrw)
	if err != nil {
		return nil, err
	}

	if msg.Tag() != BitFieldType {
		return nil, fmt.Errorf("expected BitField msg but got ID %d", msg.Tag())
	}

	return msg, nil
}

func (c *Client) announcef(format string, vars ...any) {
	if c.debug {
		peer := "unknown"
		if c.Peer != nil {
			peer = c.Peer.IP.String()
		}
		fmt.Printf("[%s] ", peer)
		fmt.Printf(format, vars...)
	}
}

func (c *Client) Have(index int) error {
	req := &Have{Index: index}
	c.announcef("Have %d index\n", index)
	data := EncodeMessage(req)
	if err := c.send(data); err != nil {
		return err
	}
	// msg, err := DecodeMessage(c.bufrw)
	// if err != nil {
	// 	return err
	// }
	//
	// c.announcef("(HAVE) recieved message %T", msg)
	return nil
}

func (c *Client) Interested() (Message, error) {
	data := EncodeMessage(&Interested{})
	if err := c.send(data); err != nil {
		return nil, err
	}
	msg, err := c.receiveMessages()
	if err != nil {
		return nil, err
	}
	if msg.Tag() != UnchokeType {
		return nil, fmt.Errorf("expected Unchoke after Interested but got %s", msg)
	}
	return msg, nil
}

func (c *Client) NotInterested() error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}

	c.announcef("sending not interested")

	return c.writeMessage(&NotInterested{})
}

func (c *Client) DownloadPiece(plan *types.BlockPlan) (*types.Piece, error) {
	defer c.announcef("<<<< End DownloadPiece [%d] >>>>\n", plan.PieceIndex)
	if c.Peer == nil {
		panic("cannot download piece with nil peer")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	msg, err := DecodeMessageWithCtx(ctx, c.bufrw)
	cancel()
	if err != nil && err != ctx.Err() {
		return nil, err
	} else if msg != nil && msg.Tag() != BitFieldType {
		return nil, fmt.Errorf("expected BitField but got %s", msg)
	}
	// 1. bitfield
	// 2. interested
	// 3. unchoke
	// 4. request
	// 5. piece
	if _, err := c.Interested(); err != nil {
		return nil, err
	}

	downloaded := make([]*PieceBlock, plan.NumBlocks)
	c.announcef("need to get %d blocks for piece %d\n", plan.NumBlocks, plan.PieceIndex)
	for i := 0; i < plan.NumBlocks; i++ {
		req := &PieceRequest{
			Index:  plan.PieceIndex,
			Begin:  i * plan.BlockSize,
			Length: plan.BlockSizeFor(i),
		}

		c.announcef("requesting %d - Begin: %d Length: %d\n", req.Index, req.Begin, req.Length)
		piece, err := c.handlePieceDownloadRequest(req)
		if err != nil {
			return nil, err
		}
		downloaded[i] = piece
	}

	c.announcef("fetched %d blocks for piece %d\n", plan.NumBlocks, plan.PieceIndex)

	data, err := assembleData(downloaded)
	if err != nil {
		return nil, err
	}
	piece := &types.Piece{
		Index: plan.PieceIndex,
		Peer:  *c.Peer,
		Size:  plan.PieceLength,
		Data:  data,
		Hash:  sha1.Sum(data),
	}

	return piece, nil
}

func (c *Client) handlePieceDownloadRequest(req *PieceRequest) (*PieceBlock, error) {
	msg, err := c.handleMessage(req)
	if err != nil {
		return nil, err
	}
	for {
		switch m := msg.(type) {
		case *Have:
			{
				if m.Index != req.Index {
					return nil, fmt.Errorf("peer does not have piece %d", req.Index)
				}
			}
		case *Unchoke:
			break
		case *PieceBlock:
			return m, nil
		}
		msg, err = c.receiveMessages()
		if err != nil {
			return nil, err
		}
	}

}

func (c *Client) receiveMessages() (Message, error) {
	do := func() (Message, error) {
		for {
			msg, err := DecodeMessage(c.bufrw)
			if err != nil {
				return nil, err
			}
			switch m := msg.(type) {
			case *KeepAlive:
				c.announcef("received keep alive\n")
			case *BitField:
				c.announcef("received bitfiled\n")
				return m, nil
			case *Choke:
				c.announcef("received choke\n")
				c.choked = true
				if err := c.waitForUnchoke(); err != nil {
					return nil, err
				}
			case *Have:
				{
					c.announcef("received have\n")
					return m, err
				}
			case *PieceBlock:
				{
					return m, nil
				}
			case *Unchoke:
				{
					return m, nil
				}
			default:
				{
					c.announcef("unknown msg received: %s\n", msg)
				}
			}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return resultWithContext(ctx, do)
}

func (c *Client) handleMessage(toSend Message) (Message, error) {
	data := EncodeMessage(toSend)
	if err := c.send(data); err != nil {
		return nil, err
	}

	return c.receiveMessages()
}
