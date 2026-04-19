package handler

import (
	"context"
	"fmt"
	"net/url"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "http-mocker/generated"
	"http-mocker/internal/mocker"
)

// TODO: stream dump and history
//
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

func (m *ManagementService) SetReply(
	_ context.Context,
	r *pb.SetReplyRequest,
) (*pb.SetReplyResponse, error) {
	requestSpec, err := m.toRequestSpec(r)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	responseSpec := m.toResponseSpec(r)
	err = m.mocker.SetReply(requestSpec, responseSpec)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "could not set reply: %v", err)
	}

	return &pb.SetReplyResponse{}, nil
}

func (m *ManagementService) toRequestSpec(
	r *pb.SetReplyRequest,
) (*mocker.RequestSpec, error) {
	u, err := url.Parse(r.Path)
	if err != nil {
		return nil, fmt.Errorf("could not parse url: %v", err)
	}

	var bodyBytes []byte
	hasBody := r.RequestBody != nil
	if hasBody {
		bodyBytes = r.RequestBody
	}

	requestSpec := mocker.NewRequestSpec(
		u.EscapedPath(), r.Method, u.Query(), hasBody, bodyBytes,
	)
	return requestSpec, nil
}

func (m *ManagementService) toResponseSpec(r *pb.SetReplyRequest) *mocker.ResponseSpec {
	var bodyBytes []byte
	hasBody := r.ResponseBody != nil
	if hasBody {
		bodyBytes = r.ResponseBody
	}

	return mocker.NewResponseSpec(int(r.StatusCode), hasBody, bodyBytes)
}

func (m *ManagementService) DumpDatabase(
	_ context.Context,
	_ *pb.DumpDatabaseRequest,
) (*pb.DumpDatabaseResponse, error) {
	runningConfig := m.mocker.DumpConfiguration()
	configView := make([]*pb.ConfigEntry, 0, len(runningConfig))
	for _, entry := range runningConfig {
		configView = append(configView, &pb.ConfigEntry{
			Req: m.toRequestSpecMessage(entry.RequestSpec()),
			Res: m.toResponseSpecMessage(entry.ResponseSpec()),
		})
	}

	return &pb.DumpDatabaseResponse{Config: configView}, nil
}

func (m *ManagementService) toRequestSpecMessage(spec *mocker.RequestSpec) *pb.RequestSpec {
	queryParams := spec.QueryParams()
	queryView := make(map[string]*pb.QueryValues, len(queryParams))
	for key, values := range queryParams {
		queryView[key] = &pb.QueryValues{Values: values}
	}

	mapped := &pb.RequestSpec{
		Path:   spec.Path(),
		Method: spec.Method(),
		Query:  queryView,
		Body:   nil,
	}

	if spec.HasBody() {
		mapped.Body = spec.Body()
	}

	return mapped
}

func (m *ManagementService) toResponseSpecMessage(spec *mocker.ResponseSpec) *pb.ResponseSpec {
	mapped := &pb.ResponseSpec{
		StatusCode: int32(spec.StatusCode()),
		Body:       nil,
	}

	if spec.HasBody() {
		mapped.Body = spec.Body()
	}

	return mapped
}

func (m *ManagementService) ClearDatabase(
	_ context.Context,
	_ *pb.ClearDatabaseRequest,
) (*pb.ClearDatabaseResponse, error) {
	m.mocker.ClearConfiguration()
	return &pb.ClearDatabaseResponse{}, nil
}

func (m *ManagementService) ListRequests(
	_ context.Context,
	_ *pb.ListRequestsRequest,
) (*pb.ListRequestsResponse, error) {
	requests := m.mocker.ListRequests()
	requestsView := make([]*pb.RequestSpec, 0, len(requests))
	for _, requestSpec := range requests {
		requestsView = append(requestsView, m.toRequestSpecMessage(requestSpec))
	}

	return &pb.ListRequestsResponse{Requests: requestsView}, nil
}
