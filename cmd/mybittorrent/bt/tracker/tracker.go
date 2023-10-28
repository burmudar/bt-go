package tracker

import "github.com/codecrafters-io/bittorrent-starter-go/cmd/mybittorrent/bt/types"

type TrackerClient struct{}

type TrackerRequest struct{}
type TrackerResponse struct{}

func (t *TrackerClient) PeerRequest(m *types.FileInfo) (*TrackerResponse, error) {
	return nil, nil
}
