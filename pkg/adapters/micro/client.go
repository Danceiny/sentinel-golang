package micro

import (
	"context"

	"github.com/micro/go-micro/v2/client"

	sentinel "github.com/Danceiny/sentinel-golang/api"
	"github.com/Danceiny/sentinel-golang/core/base"
	"github.com/Danceiny/sentinel-golang/core/outlier"
)

type clientWrapper struct {
	client.Client
	Opts []Option
}

func (c *clientWrapper) Call(ctx context.Context, req client.Request, rsp interface{}, opts ...client.CallOption) error {
	options := evaluateOptions(c.Opts)
	if options.enableOutlier == nil || !options.enableOutlier(ctx) {
		resourceName := req.Method()
		if options.clientResourceExtract != nil {
			resourceName = options.clientResourceExtract(ctx, req)
		}

		entry, blockErr := sentinel.Entry(
			resourceName,
			sentinel.WithResourceType(base.ResTypeRPC),
			sentinel.WithTrafficType(base.Outbound),
		)
		if blockErr != nil {
			if options.clientBlockFallback != nil {
				return options.clientBlockFallback(ctx, req, blockErr)
			}
			return blockErr
		}
		defer entry.Exit()

		err := c.Client.Call(ctx, req, rsp, opts...)
		if err != nil {
			sentinel.TraceError(entry, err)
		}
		return err
	} else { // returns new client middleware specifically for outlier ejection.
		slotChain := sentinel.BuildDefaultSlotChain()
		slotChain.AddRuleCheckSlot(outlier.DefaultSlot)
		slotChain.AddStatSlot(outlier.DefaultMetricStatSlot)
		entry, _ := sentinel.Entry(
			req.Service(),
			sentinel.WithResourceType(base.ResTypeRPC),
			sentinel.WithTrafficType(base.Outbound),
			sentinel.WithSlotChain(slotChain),
		)
		defer entry.Exit()
		opts = append(opts, WithSelectOption(entry))
		opts = append(opts, WithCallWrapper(entry))
		return c.Client.Call(ctx, req, rsp, opts...)
	}
}

func (c *clientWrapper) Stream(ctx context.Context, req client.Request, opts ...client.CallOption) (client.Stream, error) {
	options := evaluateOptions(c.Opts)
	if options.enableOutlier == nil || !options.enableOutlier(ctx) {
		resourceName := req.Method()
		if options.streamClientResourceExtract != nil {
			resourceName = options.streamClientResourceExtract(ctx, req)
		}

		entry, blockErr := sentinel.Entry(
			resourceName,
			sentinel.WithResourceType(base.ResTypeRPC),
			sentinel.WithTrafficType(base.Outbound),
		)
		if blockErr != nil {
			if options.streamClientBlockFallback != nil {
				return options.streamClientBlockFallback(ctx, req, blockErr)
			}
			return nil, blockErr
		}
		defer entry.Exit()

		stream, err := c.Client.Stream(ctx, req, opts...)
		if err != nil {
			sentinel.TraceError(entry, err)
		}
		return stream, err
	} else {
		slotChain := sentinel.GlobalSlotChain()
		slotChain.AddRuleCheckSlot(outlier.DefaultSlot)
		slotChain.AddStatSlot(outlier.DefaultMetricStatSlot)
		entry, _ := sentinel.Entry(
			req.Service(),
			sentinel.WithResourceType(base.ResTypeRPC),
			sentinel.WithTrafficType(base.Outbound),
			sentinel.WithSlotChain(slotChain),
		)
		defer entry.Exit()
		opts = append(opts, WithSelectOption(entry))
		opts = append(opts, WithCallWrapper(entry))
		return c.Client.Stream(ctx, req, opts...)
	}
}

// NewClientWrapper returns a sentinel client Wrapper.
func NewClientWrapper(opts ...Option) client.Wrapper {
	return func(c client.Client) client.Client {
		return &clientWrapper{c, opts}
	}
}
