package minimatch

import (
	"fmt"
	"time"

	pb "github.com/castaneai/minimatch/pkg/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type filteredEntity interface {
	GetId() string
	GetSearchFields() *pb.SearchFields
	GetCreateTime() *timestamppb.Timestamp
}

// base: https://github.com/googleforgames/open-match/blob/98e7a02ebf9e470e746265a212d9770ca353267a/internal/filter/filter.go
type PoolFilter struct {
	DoubleRangeFilters  []*pb.DoubleRangeFilter
	StringEqualsFilters []*pb.StringEqualsFilter
	TagPresentFilters   []*pb.TagPresentFilter
	CreatedBefore       time.Time
	CreatedAfter        time.Time
}

func NewPoolFilter(pool *pb.Pool) (*PoolFilter, error) {
	var ca, cb time.Time
	var err error

	if pool.GetCreatedBefore() != nil {
		if err = pool.GetCreatedBefore().CheckValid(); err != nil {
			return nil, fmt.Errorf("invalid created_before value")
		}
		cb = pool.GetCreatedBefore().AsTime()
	}

	if pool.GetCreatedAfter() != nil {
		if err = pool.GetCreatedAfter().CheckValid(); err != nil {
			return nil, fmt.Errorf("invalid created_after value")
		}
		ca = pool.GetCreatedAfter().AsTime()
	}

	return &PoolFilter{
		DoubleRangeFilters:  pool.GetDoubleRangeFilters(),
		StringEqualsFilters: pool.GetStringEqualsFilters(),
		TagPresentFilters:   pool.GetTagPresentFilters(),
		CreatedBefore:       cb,
		CreatedAfter:        ca,
	}, nil
}

func (pf *PoolFilter) In(entity filteredEntity) bool {
	s := entity.GetSearchFields()

	if s == nil {
		s = &pb.SearchFields{}
	}

	if !pf.CreatedAfter.IsZero() || !pf.CreatedBefore.IsZero() {
		// CreateTime is only populated by Open Match and hence expected to be valid.
		if err := entity.GetCreateTime().CheckValid(); err == nil {
			ct := entity.GetCreateTime().AsTime()
			if !pf.CreatedAfter.IsZero() {
				if !ct.After(pf.CreatedAfter) {
					return false
				}
			}

			if !pf.CreatedBefore.IsZero() {
				if !ct.Before(pf.CreatedBefore) {
					return false
				}
			}
		} else {
		}
	}

	for _, f := range pf.DoubleRangeFilters {
		v, ok := s.DoubleArgs[f.DoubleArg]
		if !ok {
			return false
		}

		switch f.Exclude {
		case pb.DoubleRangeFilter_NONE:
			// Not simplified so that NaN cases are handled correctly.
			if !(v >= f.Min && v <= f.Max) {
				return false
			}
		case pb.DoubleRangeFilter_MIN:
			if !(v > f.Min && v <= f.Max) {
				return false
			}
		case pb.DoubleRangeFilter_MAX:
			if !(v >= f.Min && v < f.Max) {
				return false
			}
		case pb.DoubleRangeFilter_BOTH:
			if !(v > f.Min && v < f.Max) {
				return false
			}
		}

	}

	for _, f := range pf.StringEqualsFilters {
		v, ok := s.StringArgs[f.StringArg]
		if !ok {
			return false
		}
		if f.Value != v {
			return false
		}
	}

outer:
	for _, f := range pf.TagPresentFilters {
		for _, v := range s.Tags {
			if v == f.Tag {
				continue outer
			}
		}
		return false
	}

	return true
}
