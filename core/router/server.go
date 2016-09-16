// Copyright © 2016 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package router

import (
	"io"

	pb "github.com/TheThingsNetwork/ttn/api/router"
	"github.com/TheThingsNetwork/ttn/utils/errors"
	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
)

type routerRPC struct {
	router Router
}

var grpcErrf = grpc.Errorf // To make go vet stop complaining

func metadataFromContext(ctx context.Context) (md metadata.MD, err error) {
	var ok bool
	if md, ok = metadata.FromContext(ctx); !ok {
		return md, errors.NewErrInternal("Could not get metadata from context")
	}
	return md, nil
}

func gatewayFromContext(ctx context.Context) (gatewayID string, err error) {
	md, err := metadataFromContext(ctx)
	if err != nil {
		return
	}

	return getGatewayFromMetadata(md)
}

func gatewayFromMetadata(md metadata.MD) (gatewayID string, err error) {
	id, ok := md["id"]
	if !ok || len(id) < 1 {
		err = errors.NewErrInvalidArgument("Metadata", "id missing")
		return
	}
	return id[0], nil
}

func getTokenFromMetadata(md metadata.MD) (string, error) {
	token, ok := md["token"]
	if !ok || len(token) < 1 {
		return "", errors.NewErrInvalidArgument("Metadata", "token missing")
	}

	if token[0] != "token" {
		// TODO: Validate Token
		return "", errors.NewErrPermissionDenied("Gateway token not authorized")
	}
	return token[0], nil
}

// GatewayStatus implements RouterServer interface (github.com/TheThingsNetwork/ttn/api/router)
func (r *routerRPC) GatewayStatus(stream pb.Router_GatewayStatusServer) error {
	md, err := metadataFromContext(stream.Context())

	id, err := getGatewayFromMetadata(md)
	if err != nil {
		return errors.BuildGRPCError(err)
	}

	token, err := getTokenFromMetadata(md)
	if err != nil {
		return err
	}

	//TODO Validate token

	//r.router.getGateway(id).Token = token
	r.router.getGateway(id).Token = token // FIXME

	for {
		status, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&empty.Empty{})
		}
		if err != nil {
			return err
		}
		if !status.Validate() {
			return grpcErrf(codes.InvalidArgument, "Invalid Gateway Status")
		}
		go r.router.HandleGatewayStatus(id, status)
	}
}

// Uplink implements RouterServer interface (github.com/TheThingsNetwork/ttn/api/router)
func (r *routerRPC) Uplink(stream pb.Router_UplinkServer) error {
	md, err := metadataFromContext(stream.Context())

	id, err := getGatewayFromMetadata(md)
	if err != nil {
		return err
	}

	token, err := getTokenFromMetadata(md)
	if err != nil {
		return errors.BuildGRPCError(err)
	}

	//TODO Validate token

	//r.router.getGateway(id).Token = token
	r.router.getGateway(id).Token = token // FIXME

	for {
		uplink, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&empty.Empty{})
		}
		if err != nil {
			return err
		}
		if !uplink.Validate() {
			return grpcErrf(codes.InvalidArgument, "Invalid Uplink")
		}
		go r.router.HandleUplink(id, uplink)
	}
}

// Subscribe implements RouterServer interface (github.com/TheThingsNetwork/ttn/api/router)
func (r *routerRPC) Subscribe(req *pb.SubscribeRequest, stream pb.Router_SubscribeServer) error {
	md, err := metadataFromContext(stream.Context())

	id, err := getGatewayFromMetadata(md)
	if err != nil {
		return err
	}

	// TODO validate token

	downlinkChannel, err := r.router.SubscribeDownlink(id)
	if err != nil {
		return err
	}
	defer r.router.UnsubscribeDownlink(id)

	for {
		if downlinkChannel == nil {
			return nil
		}
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case downlink := <-downlinkChannel:
			if err := stream.Send(downlink); err != nil {
				return err
			}
		}
	}
}

// Activate implements RouterServer interface (github.com/TheThingsNetwork/ttn/api/router)
func (r *routerRPC) Activate(ctx context.Context, req *pb.DeviceActivationRequest) (*pb.DeviceActivationResponse, error) {
	md, err := metadataFromContext(ctx)

	id, err := getGatewayFromMetadata(md)
	if err != nil {
		return nil, err
	}
	if !req.Validate() {
		return nil, grpcErrf(codes.InvalidArgument, "Invalid Activation Request")
	}
	return r.router.HandleActivation(id, req)
}

// RegisterRPC registers this router as a RouterServer (github.com/TheThingsNetwork/ttn/api/router)
func (r *router) RegisterRPC(s *grpc.Server) {
	server := &routerRPC{r}
	pb.RegisterRouterServer(s, server)
}
