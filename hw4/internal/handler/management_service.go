package handler

import (
	"context"
	"fmt"
	"net/url"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "http-mocker/generated"
	"http-mocker/pkg/mocker"
)

//go:generate protoc -I=../../protos --go_out=../../generated --go-grpc_out=../../generated ../../protos/management_service.proto

// ManagementService implements the GRPC API for configuring the HttpMocker.
type ManagementService struct {

	// mocker is the core engine instance being configured
	mocker *mocker.HttpMocker

	pb.UnimplementedManagementServiceServer
}

// NewManagementService constructs a new instance of ManagementService
func NewManagementService(mocker *mocker.HttpMocker) *ManagementService {
	return &ManagementService{
		mocker: mocker,
	}
}

// SetReply parse GRPC request ot mocker's RequestSpec/ResponseSpec
// and updates the underlying mocker instance.
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
		return nil, status.Errorf(
			codes.InvalidArgument, "could not set reply: %v", err,
		)
	}

	return &pb.SetReplyResponse{}, nil
}

// toRequestSpec converts a pb.SetReplyRequest to an internal mocker.RequestSpec.
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

// toResponseSpec converts a pb.SetReplyRequest to an internal mocker.ResponseSpec.
func (m *ManagementService) toResponseSpec(
	r *pb.SetReplyRequest,
) *mocker.ResponseSpec {
	var bodyBytes []byte
	hasBody := r.ResponseBody != nil
	if hasBody {
		bodyBytes = r.ResponseBody
	}

	headers := make(map[string][]string, len(r.ResponseHeaders))
	for key, pbValues := range r.ResponseHeaders {
		headers[key] = pbValues.Values
	}

	return mocker.NewResponseSpec(
		int(r.StatusCode),
		headers,
		hasBody,
		bodyBytes,
	)
}

// DumpDatabase retrieves running configuration from
// underlying mocker instance, and builds response.
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

// toRequestSpecMessage maps mocker's RequestSpec object into GRPC message
func (m *ManagementService) toRequestSpecMessage(
	spec *mocker.RequestSpec,
) *pb.RequestSpec {
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

// toResponseSpecMessage maps mocker's RequestSpec object into GRPC message
func (m *ManagementService) toResponseSpecMessage(
	spec *mocker.ResponseSpec,
) *pb.ResponseSpec {
	headers := spec.Headers()
	pbHeaders := make(map[string]*pb.HeaderValues, len(headers))
	for key, values := range headers {
		pbHeaders[key] = &pb.HeaderValues{Values: values}
	}

	mapped := &pb.ResponseSpec{
		StatusCode: int32(spec.StatusCode()),
		Headers:    pbHeaders,
		Body:       nil,
	}

	if spec.HasBody() {
		mapped.Body = spec.Body()
	}
	return mapped
}

// ClearDatabase triggers the mocker to remove all configured rules.
func (m *ManagementService) ClearDatabase(
	_ context.Context,
	_ *pb.ClearDatabaseRequest,
) (*pb.ClearDatabaseResponse, error) {
	m.mocker.ClearConfiguration()
	return &pb.ClearDatabaseResponse{}, nil
}

// ListRequests retrieves the history of received HTTP requests from
// the mocker and transforms them into a gRPC response.
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
