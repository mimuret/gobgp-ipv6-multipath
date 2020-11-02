package main

import (
	"context"
	"fmt"
	"io"
	"log"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	api "github.com/osrg/gobgp/api"
	"github.com/osrg/gobgp/pkg/server"
	gobgp "github.com/osrg/gobgp/pkg/server"
	"google.golang.org/grpc"
)

func main() {
	s := gobgp.NewBgpServer(server.GrpcListenAddress("127.0.0.1:50051"))
	go s.Serve()
	defer s.StopBgp(context.Background(), &api.StopBgpRequest{})
	if err := s.StartBgp(context.Background(), &api.StartBgpRequest{
		Global: &api.Global{
			As:               65003,
			RouterId:         "10.0.255.254",
			ListenPort:       -1, // gobgp won't listen on tcp:179
			Families:         []uint32{uint32(0), uint32(1)},
			UseMultiplePaths: true,
		},
	}); err != nil {
		log.Fatal(err)
	}
	grpcOpts := []grpc.DialOption{grpc.WithBlock(), grpc.WithInsecure()}
	conn, err := grpc.DialContext(context.Background(), "127.0.0.1:50051", grpcOpts...)
	client := api.NewGobgpApiClient(conn)
	if err != nil {
		log.Fatal(err)
	}
	v6UniFamily := &api.Family{
		Afi:  api.Family_AFI_IP6,
		Safi: api.Family_SAFI_UNICAST,
	}

	nexthops := []string{"2001:db8::1", "2001:db8::2"}
	nlri, _ := ptypes.MarshalAny(&api.IPAddressPrefix{
		Prefix:    "2001:db8::",
		PrefixLen: 48,
	})
	for i, nexthop := range nexthops {
		a1, _ := ptypes.MarshalAny(&api.OriginAttribute{
			Origin: 0, // ibgp
		})
		a2, _ := ptypes.MarshalAny(&api.MpReachNLRIAttribute{
			Family:   v6UniFamily,
			NextHops: []string{nexthop},
			Nlris:    []*any.Any{nlri},
		})
		attrs := []*any.Any{a1, a2}
		path := &api.Path{
			Family:     v6UniFamily,
			Nlri:       nlri,
			Pattrs:     attrs,
			Identifier: uint32(i + 100),
		}
		_, err := client.AddPath(context.Background(), &api.AddPathRequest{
			TableType: api.TableType_GLOBAL,
			Path:      path,
		})
		if err != nil {
			log.Fatal(err)
		}
	}
	listStream, err := client.ListPath(context.Background(), &api.ListPathRequest{
		TableType: api.TableType_GLOBAL,
		Family:    v6UniFamily,
	})
	for {
		r, err := listStream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatal(err)
		}
		dst := r.Destination
		p := dst.GetPaths()
		for _, path := range p {
			fmt.Println(path)
		}
	}
}
