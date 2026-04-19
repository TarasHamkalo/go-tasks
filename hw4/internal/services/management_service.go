package services

import (
	"context"
	pb "http-mocker/generated"
	"http-mocker/internal/mocker"
	"net/url"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

//go:generate protoc -I=../../protos --go_out=../../generated --go-grpc_out=../../generated ../../protos/management_service.proto
type ManagementService struct {
	mocker *mocker.HttpMocker

	pb.UnimplementedManagementServiceServer
}

func NewManagementService(mocker *mocker.HttpMocker) *ManagementService {
	return &ManagementService{
		mocker: mocker,
	}
}

func (m *ManagementService) SetReply(ctx context.Context, r *pb.SetReplyRequest) (*pb.SetReplyResponse, error) {
	u, err := url.Parse(r.Path)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "could not parse url: %v", err)
	}

	requestSpecBuilder := mocker.NewRequestSpecBuilder(u.EscapedPath(), r.Method).
		WithQueryParams(u.Query())

	if r.RequestBody != nil && len(r.RequestBody) > 0 {
		// leaving nil body = empty body
		// http client might send Content-Length: 0 even though body was not set at all
		requestSpecBuilder.
			WithBody(append([]byte{}, r.RequestBody...))
	}

	requestSpec := requestSpecBuilder.Build()
	responseSpec := mocker.NewResponseSpec(
		int(r.StatusCode),
		r.ResponseBody,
	)

	err = m.mocker.SetReply(requestSpec, responseSpec)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "could not set reply: %v", err)
	}

	return &pb.SetReplyResponse{}, nil
}

//
//func (UnimplementedManagementServiceServer) DumpDatabase(context.Context, *DumpDatabaseRequest) (*DumpDatabaseResponse, error) {
//	return nil, status.Error(codes.Unimplemented, "method DumpDatabase not implemented")
//}
//func (UnimplementedManagementServiceServer) ClearDatabase(context.Context, *ClearDatabaseRequest) (*ClearDatabaseResponse, error) {
//	return nil, status.Error(codes.Unimplemented, "method ClearDatabase not implemented")
//}
//func (UnimplementedManagementServiceServer) ListRequests(context.Context, *ListRequestsRequest) (*ListRequestsResponse, error) {
//	return nil, status.Error(codes.Unimplemented, "method ListRequests not implemented")
//}
