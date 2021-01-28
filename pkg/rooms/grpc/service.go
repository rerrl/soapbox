package grpc

import (
	"context"

	"github.com/soapboxsocial/soapbox/pkg/rooms"
	"github.com/soapboxsocial/soapbox/pkg/rooms/pb"
)

type Service struct {
	repository *rooms.Repository
}

func NewService(repository *rooms.Repository) *Service {
	return &Service{
		repository: repository,
	}
}

func (s *Service) GetRoom(ctx context.Context, in *pb.RoomQuery) (*pb.RoomResponse, error) {
	r, err := s.repository.Get(in.Id)
	if err != nil {
		return &pb.RoomResponse{Room: nil, Error: "not found"}, nil
	}

	return &pb.RoomResponse{Room: r.ToProto(), Error: ""}, nil
}