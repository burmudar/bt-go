package tracker

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/codecrafters-io/bittorrent-starter-go/cmd/mybittorrent/bt"
	"github.com/codecrafters-io/bittorrent-starter-go/cmd/mybittorrent/bt/encoding"
	"github.com/codecrafters-io/bittorrent-starter-go/cmd/mybittorrent/bt/types"
)

type TrackerClient struct {
	client *http.Client
}

type TrackerRequest interface {
	HTTPRequest() (*http.Request, error)
}

type PeersRequest struct {
	Announce string
	//info_hash: the info hash of the torrent
	// 20 bytes long, will need to be URL encoded
	InfoHash [20]byte
	// peer_id: a unique identifier for your client
	// A string of length 20 that you get to pick. You can use something like 00112233445566778899.
	PeerID string
	// port: the port your client is listening on
	// You can set this to 6881, you will not have to support this functionality during this challenge.
	Port int
	// uploaded: the total amount uploaded so far
	// Since your client hasn't uploaded anything yet, you can set this to 0.
	Uploaded float64
	// downloaded: the total amount downloaded so far
	// Since your client hasn't downloaded anything yet, you can set this to 0.
	Downloaded float64
	// left: the number of bytes left to download
	// Since you client hasn't downloaded anything yet, this'll be the total length of the file (you've extracted this value from the torrent file in previous stages)
	Left int
	// compact: whether the peer list should use the compact representation
	// For the purposes of this challenge, set this to 1.
	// The compact representation is more commonly used in the wild, the non-compact representation is mostly supported for backward-compatibility.
	Compact int
}

type PeersResponse struct {
	Interval int
	Peers    []*types.Peer
}

func NewClient() *TrackerClient {
	return &TrackerClient{
		client: http.DefaultClient,
	}
}

func NewPeerRequest(peerID string, port int, m *types.FileMeta) (*PeersRequest, error) {
	infoHash, err := bt.InfoHash(m)
	if err != nil {
		return nil, err
	}
	return &PeersRequest{
		Announce: m.Announce,
		PeerID:   peerID,
		Port:     port,
		InfoHash: infoHash,
		Left:     m.Length,
		Compact:  1,
	}, nil

}

func percentEncode(data []byte) string {
	var builder strings.Builder

	for _, b := range data {
		if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') ||
			b == '-' || b == '_' || b == '.' || b == '~' {
			builder.WriteByte(b)
		} else {
			builder.WriteString(fmt.Sprintf("%%%d", b))
		}
	}

	return builder.String()
}

func (p *PeersRequest) HTTPRequest() (*http.Request, error) {
	reqValues := url.Values{}
	reqValues.Set("info_hash", string(p.InfoHash[:]))
	reqValues.Set("peer_id", p.PeerID)
	reqValues.Set("port", fmt.Sprintf("%d", p.Port))
	reqValues.Set("uploaded", "0")
	reqValues.Set("downloaded", "0")
	reqValues.Set("left", fmt.Sprintf("%d", p.Left))
	reqValues.Set("compact", fmt.Sprintf("%d", p.Compact))

	trackerURL, err := url.Parse(p.Announce)
	if err != nil {
		return nil, err
	}
	trackerURL.RawQuery = reqValues.Encode()

	fmt.Println(trackerURL.String())

	return http.NewRequest("GET", trackerURL.String(), nil)
}

func (t *TrackerClient) PeersRequest(treq TrackerRequest) (*PeersResponse, error) {
	req, err := treq.HTTPRequest()
	if err != nil {
		return nil, fmt.Errorf("http request creation failure: %v", err)
	}

	resp, err := t.client.Do(req)
	if err != err {
		return nil, err
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	presp, err := decodePeersResponse(data)
	return presp, err
}

func decodePeersResponse(d []byte) (*PeersResponse, error) {
	r := encoding.NewBencodeReader(string(d))
	v, err := encoding.DecodeBencode(r)
	if err != nil {
		return nil, err
	}

	dict, ok := v.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to cast value of %T to dictionary", v)
	}

	rawErrReason, ok := dict["failure reason"]
	if ok {
		return nil, fmt.Errorf(rawErrReason.(string))
	}

	rawInterval, ok := dict["interval"]
	if !ok {
		return nil, fmt.Errorf("malformed peers response - missing 'interval' key")
	}
	interval := rawInterval.(int)

	rawPeers, ok := dict["peers"]
	if !ok {
		return nil, fmt.Errorf("malformed peers response - missing 'peers' key")
	}

	// parse peers
	// Each peer is represented using 6 bytes.
	// The first 4 bytes are the peer's IP address and the last 2 bytes are the peer's port number.
	peerData := rawPeers.(string)
	br := bytes.NewReader([]byte(peerData))

	var peerBuf [6]byte
	peers := []*types.Peer{}
	var readErr error
	for readErr != io.EOF && readErr == nil {
		_, readErr = br.Read(peerBuf[:])

		port, err := strconv.Atoi(string(peerBuf[4:6]))
		if err != nil {
			return nil, fmt.Errorf("error converting port bytes to int: %v", err)
		}

		peers = append(peers, &types.Peer{
			IP:   net.IPv4(peerBuf[0], peerBuf[1], peerBuf[2], peerBuf[3]),
			Port: port,
		})
	}

	if readErr != io.EOF && readErr != nil {
		return nil, fmt.Errorf("failed to read peer bytes: %v", readErr)
	}

	return &PeersResponse{
		Interval: interval,
		Peers:    peers,
	}, nil

}
