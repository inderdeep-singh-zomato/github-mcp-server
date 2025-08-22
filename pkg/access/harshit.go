package access

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/Zomato/resource-map-service-client-golang/proto/resource-map-service"
)

type repository struct {
	repo string
	org  string
}

func (r *repository) GetOrg() string {
	if r == nil {
		return ""
	}

	return r.org
}

func (r *repository) GetRepo() string {
	if r == nil {
		return ""
	}

	return r.repo
}

func newConnection(authority, host string, tlsEnabled bool) (*grpc.ClientConn, error) {
	conn, err := grpc.NewClient(host, grpcDialOptions(authority, tlsEnabled)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection: %v", err)
	}
	return conn, nil
}

func loadSystemRootCAs() (*x509.CertPool, error) {
	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}
	return pool, nil
}

func grpcDialOptions(authority string, tlsEnabled bool) []grpc.DialOption {
	var opts []grpc.DialOption

	if authority != "" {
		opts = append(opts, grpc.WithAuthority(authority))
	}

	if !tlsEnabled {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		rootCAs, err := loadSystemRootCAs()
		if err != nil {
			panic(fmt.Errorf("failed to load root CAs: %w", err))
		}

		tlsConfig := &tls.Config{
			RootCAs: rootCAs,
		}

		creds := credentials.NewTLS(tlsConfig)
		opts = append(opts, grpc.WithTransportCredentials(creds))
	}

	return opts
}

func initialise(authority, endpoint string, tlsEnabled bool) (pb.SyncResourceMapClient, error) {
	conn, err := newConnection(authority, endpoint, tlsEnabled)
	if err != nil {
		log.Fatalf("Error creating connection: %v\n", err)
		return nil, err
	}
	rmapClient, err := pb.NewSyncClient(conn)
	if err != nil {
		log.Fatalf("Error creating resource map client: %v", err)
		return nil, err
	}
	return rmapClient, nil
}

func GetAllAccessibleRepos(email string) ([]repository, error) {
	client, err := initialise("", "host.docker.internal:3000", false)
	if err != nil {
		return nil, err
	}

	res, err := client.Traverse(context.Background(), &pb.TraverseRequest{
		SourceNodeType: "zomato/member",
		SourceNodeLabels: []string{
			"zomato/member",
		},
		SourceNodeProperties: map[string]string{
			"name": email,
		},
		Relations: []*pb.TraverseRelation{
			{
				RelationType:     "memberOf",
				TargetNodeType:   "github/team",
				TargetNodeLabels: []string{"github/team"},
			},
			{
				RelationType:     "accessTo",
				TargetNodeType:   "github/repository",
				TargetNodeLabels: []string{"github/repository"},
			},
		},
		OutputType: "github/repository",
	})
	if err != nil {
		return nil, err
	}

	accessibleRepos := []repository{}
	for _, output := range res.Output {
		properties := output.GetNode().GetProperties()

		var repo, org string
		if val, ok := properties["org"]; ok {
			org = val
		}
		if val, ok := properties["name"]; ok {
			repo = val
		}

		if repo != "" && org != "" {
			accessibleRepos = append(accessibleRepos, repository{
				repo: repo,
				org:  org,
			})
		}
	}

	return accessibleRepos, nil
}