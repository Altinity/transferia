package consumer

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	kinesistypes "github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	"github.com/transferia/transferia/internal/logger"
	"github.com/transferia/transferia/library/go/core/xerrors"
	yslices "github.com/transferia/transferia/library/go/slices"
	"go.ytsaurus.tech/library/go/core/log"
)

// Record wraps the record returned from the Kinesis library and
// extends to include the shard id.
type Record struct {
	kinesistypes.Record
	ShardID            string
	MillisBehindLatest *int64
}

type KinesisAPI interface {
	GetRecords(ctx context.Context, params *kinesis.GetRecordsInput, optFns ...func(*kinesis.Options)) (*kinesis.GetRecordsOutput, error)
	GetShardIterator(ctx context.Context, params *kinesis.GetShardIteratorInput, optFns ...func(*kinesis.Options)) (*kinesis.GetShardIteratorOutput, error)
	ListShards(ctx context.Context, params *kinesis.ListShardsInput, optFns ...func(*kinesis.Options)) (*kinesis.ListShardsOutput, error)
}

func New(streamName string, opts ...Option) (*Consumer, error) {
	if streamName == "" {
		return nil, xerrors.New("must provide stream name")
	}

	c := &Consumer{
		streamName:               streamName,
		initialShardIteratorType: kinesistypes.ShardIteratorTypeLatest,
		initialTimestamp:         nil,
		client:                   nil,
		group:                    nil,
		logger:                   logger.Log,
		store:                    &noopStore{},
		scanInterval:             5 * time.Millisecond,
		maxRecords:               10000,
		shardClosedHandler:       nil,
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.client == nil {
		cfg, err := awsconfig.LoadDefaultConfig(context.Background())
		if err != nil {
			return nil, err
		}
		c.client = kinesis.NewFromConfig(cfg)
	}

	if c.group == nil {
		c.group = NewAllGroup(c.client, c.store, streamName, c.logger)
	}

	return c, nil
}

type Consumer struct {
	streamName               string
	initialShardIteratorType kinesistypes.ShardIteratorType
	initialTimestamp         *time.Time
	client                   KinesisAPI
	group                    Group
	logger                   log.Logger
	store                    Store
	scanInterval             time.Duration
	maxRecords               int32
	shardClosedHandler       ShardClosedHandler
}

type ScanFunc func([]*Record) error

func (c *Consumer) SetCheckpoint(shardID, sequenceNumber string) error {
	return c.store.SetCheckpoint(c.streamName, shardID, sequenceNumber)
}

// Scan launches a goroutine to process each of the shards in the stream. The ScanFunc
// is passed through to each of the goroutines and called with each message pulled from
// the stream.
func (c *Consumer) Scan(ctx context.Context, fn ScanFunc) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		errc   = make(chan error, 1)
		shardc = make(chan kinesistypes.Shard, 1)
	)

	go func() {
		c.group.Start(ctx, shardc)
		<-ctx.Done()
		close(shardc)
	}()

	wg := new(sync.WaitGroup)
	// process each of the shards
	for shard := range shardc {
		wg.Add(1)
		go func(shardID string) {
			defer wg.Done()
			if err := c.ScanShard(ctx, shardID, fn); err != nil {
				select {
				case errc <- xerrors.Errorf("shard %s error: %w", shardID, err):
					// first error to occur
					cancel()
				default:
					// error has already occurred
				}
			}
		}(aws.ToString(shard.ShardId))
	}

	go func() {
		wg.Wait()
		close(errc)
	}()

	return <-errc
}

// ScanShard loops over records on a specific shard, calls the callback func
// for each record and checkpoints the progress of scan.
func (c *Consumer) ScanShard(ctx context.Context, shardID string, fn ScanFunc) error {
	// get last seq number from checkpoint
	lastSeqNum, err := c.group.GetCheckpoint(c.streamName, shardID)
	if err != nil {
		return xerrors.Errorf("get checkpoint error: %w", err)
	}

	// get shard iterator
	shardIterator, err := c.getShardIterator(ctx, c.streamName, shardID, lastSeqNum)
	if err != nil {
		return xerrors.Errorf("get shard iterator error: %w", err)
	}

	c.logger.Infof("start scan: %s / %v", shardID, lastSeqNum)
	defer func() {
		c.logger.Infof("stop scan: %s", shardID)
	}()
	scanTicker := time.NewTicker(c.scanInterval)
	defer scanTicker.Stop()

	for {
		resp, err := c.client.GetRecords(ctx, &kinesis.GetRecordsInput{
			Limit:         aws.Int32(c.maxRecords),
			ShardIterator: shardIterator,
		})
		// attempt to recover from GetRecords error when expired iterator
		if err != nil {
			c.logger.Warn("get records error", log.Error(err))

			if !isRetriableError(err) {
				return xerrors.Errorf("get records error: %w", err)
			}

			shardIterator, err = c.getShardIterator(ctx, c.streamName, shardID, lastSeqNum)
			if err != nil {
				return xerrors.Errorf("get shard iterator error: %w", err)
			}
			continue
		}

		err = fn(yslices.Map(resp.Records, func(r kinesistypes.Record) *Record {
			lastSeqNum = aws.ToString(r.SequenceNumber)
			return &Record{Record: r, ShardID: shardID, MillisBehindLatest: resp.MillisBehindLatest}
		}))
		if err != nil {
			return xerrors.Errorf("unable to process records: %w", err)
		}

		if isShardClosed(resp.NextShardIterator, shardIterator) {
			c.logger.Infof("shard closed %s", shardID)
			if c.shardClosedHandler != nil {
				err := c.shardClosedHandler(c.streamName, shardID)
				if err != nil {
					return xerrors.Errorf("shard closed handler error: %w", err)
				}
			}
			return nil
		}

		shardIterator = resp.NextShardIterator

		// Wait for next scan
		select {
		case <-ctx.Done():
			return nil
		case <-scanTicker.C:
			continue
		}
	}
}

func isRetriableError(err error) bool {
	var expired *kinesistypes.ExpiredIteratorException
	if errors.As(err, &expired) {
		return true
	}
	var throughput *kinesistypes.ProvisionedThroughputExceededException
	if errors.As(err, &throughput) {
		return true
	}
	var internalFailure *kinesistypes.InternalFailureException
	return errors.As(err, &internalFailure)
}

func isShardClosed(nextShardIterator, currentShardIterator *string) bool {
	return nextShardIterator == nil || currentShardIterator == nextShardIterator
}

func (c *Consumer) getShardIterator(ctx context.Context, streamName, shardID, seqNum string) (*string, error) {
	params := &kinesis.GetShardIteratorInput{
		ShardId:    aws.String(shardID),
		StreamName: aws.String(streamName),
	}

	if seqNum != "" {
		params.ShardIteratorType = kinesistypes.ShardIteratorTypeAfterSequenceNumber
		params.StartingSequenceNumber = aws.String(seqNum)
	} else if c.initialTimestamp != nil {
		params.ShardIteratorType = kinesistypes.ShardIteratorTypeAtTimestamp
		params.Timestamp = c.initialTimestamp
	} else {
		params.ShardIteratorType = c.initialShardIteratorType
	}

	res, err := c.client.GetShardIterator(ctx, params)
	if err != nil {
		return nil, err
	}
	return res.ShardIterator, err
}
