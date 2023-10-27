package grpc

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/stefanprodan/podinfo/pkg/grpc/header"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type headerServer struct {
	header.UnimplementedHeaderServiceServer
}

const (
	timestampFormat = time.StampNano
	streamingCount  = 10
)


func (s *headerServer) Header (ctx context.Context, in *header.HeaderRequest) (*header.HeaderResponse, error) {
	//md, ok := metadata.FromIncomingContext
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.DataLoss, "UnaryEcho: failed to get metadata")
	}
	// if t, ok := md["timestamp"]; ok {
	// 	fmt.Printf("timestamp from metadata:\n")
	// 	for i, e := range t {
	// 		fmt.Printf(" %d. %s\n", i, e)
	// 	}
	// }

	// fmt.Printf("metaData \n %v", md)
	// fmt.Printf("\n metaData Length %d \n", len(md))
	// fmt.Printf("\n metaData Length %v \n", md[0])


	var res string
	// var strArr [100]string
	var cnt = 0
	// m := make(map[string]string)
	for i, e := range md {
		fmt.Printf(" %s %s\n", i, strings.Join(e, ","))
		// m[i] =  strings.Join(e, ",")
		// strArr[cnt] = i + " : " + strings.Join(e, ",")
		res = res + " , "+ i + " : " + strings.Join(e, ",")
		cnt++
	}
	fmt.Printf("\n metaData \n %v", res)

	// fmt.Printf("request received: %v, sending echo\n", in)

	return &header.HeaderResponse{Header: res} , nil
	// return nil , nil

}