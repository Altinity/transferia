package consumer

import (
	"context"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	kinesistypes "github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	"github.com/transferia/transferia/library/go/core/xerrors"
	"go.ytsaurus.tech/library/go/core/log"
)

// NewAllGroup returns an intitialized AllGroup for consuming
// all shards on a stream
func NewAllGroup(ksis KinesisAPI, store Store, streamName string, logger log.Logger) *AllGroup {
	return &AllGroup{
		Store:      store,
		ksis:       ksis,
		streamName: streamName,
		logger:     logger,
		shardMu:    sync.Mutex{},
		shards:     make(map[string]kinesistypes.Shard),
	}
}

// AllGroup is used to consume all shards from a single consumer. It
// caches a local list of the shards we are already processing
// and routinely polls the stream looking for new shards to process.
type AllGroup struct {
	Store

	ksis       KinesisAPI
	streamName string
	logger     log.Logger

	shardMu sync.Mutex
	shards  map[string]kinesistypes.Shard
}

// Start is a blocking operation which will loop and attempt to find new
// shards on a regular cadence.
func (g *AllGroup) Start(ctx context.Context, shardc chan kinesistypes.Shard) {
	var ticker = time.NewTicker(30 * time.Second)
	g.findNewShards(ctx, shardc)

	// Note: while ticker is a rather naive approach to this problem,
	// it actually simplies a few things. i.e. If we miss a new shard while
	// AWS is resharding we'll pick it up max 30 seconds later.

	// It might be worth refactoring this flow to allow the consumer to
	// to notify the broker when a shard is closed. However, shards don't
	// necessarily close at the same time, so we could potentially get a
	// thundering heard of notifications from the consumer.

	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			return
		case <-ticker.C:
			g.findNewShards(ctx, shardc)
		}
	}
}

// findNewShards pulls the list of shards from the Kinesis API
// and uses a local cache to determine if we are already processing
// a particular shard.
func (g *AllGroup) findNewShards(ctx context.Context, shardc chan kinesistypes.Shard) {
	g.shardMu.Lock()
	defer g.shardMu.Unlock()

	shards, err := listShards(ctx, g.ksis, g.streamName)
	if err != nil {
		g.logger.Warn("list shard failed error", log.Error(err))
		return
	}

	for _, shard := range shards {
		shardID := aws.ToString(shard.ShardId)
		if _, ok := g.shards[shardID]; ok {
			continue
		}
		g.shards[shardID] = shard
		shardc <- shard
	}
}

// listShards pulls a list of shard IDs from the kinesis api
func listShards(ctx context.Context, ksis KinesisAPI, streamName string) ([]kinesistypes.Shard, error) {
	var ss []kinesistypes.Shard
	var listShardsInput = &kinesis.ListShardsInput{
		StreamName: aws.String(streamName),
	}

	for {
		resp, err := ksis.ListShards(ctx, listShardsInput)
		if err != nil {
			return nil, xerrors.Errorf("ListShards failed: %w", err)
		}
		ss = append(ss, resp.Shards...)

		if resp.NextToken == nil {
			return ss, nil
		}

		listShardsInput = &kinesis.ListShardsInput{
			NextToken: resp.NextToken,
		}
	}
}
