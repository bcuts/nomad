package scheduler

import (
	"testing"

	"fmt"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestResourceDistance(t *testing.T) {
	resourceAsk := &structs.Resources{
		CPU:      2048,
		MemoryMB: 512,
		IOPS:     300,
		DiskMB:   4096,
		Networks: []*structs.NetworkResource{
			{
				Device: "eth0",
				MBits:  1024,
			},
		},
	}

	type testCase struct {
		allocResource    *structs.Resources
		expectedDistance string
	}

	testCases := []*testCase{
		{
			&structs.Resources{
				CPU:      2048,
				MemoryMB: 512,
				IOPS:     300,
				DiskMB:   4096,
				Networks: []*structs.NetworkResource{
					{
						Device: "eth0",
						MBits:  1024,
					},
				},
			},
			"0.000",
		},
		{
			&structs.Resources{
				CPU:      1024,
				MemoryMB: 400,
				IOPS:     200,
				DiskMB:   1024,
				Networks: []*structs.NetworkResource{
					{
						Device: "eth0",
						MBits:  1024,
					},
				},
			},
			"0.986",
		},
		{
			&structs.Resources{
				CPU:      1024,
				MemoryMB: 200,
				IOPS:     200,
				DiskMB:   1024,
				Networks: []*structs.NetworkResource{
					{
						Device: "eth0",
						MBits:  512,
					},
				},
			},
			"1.243",
		},
		{
			&structs.Resources{
				CPU:      8192,
				MemoryMB: 200,
				IOPS:     200,
				DiskMB:   1024,
				Networks: []*structs.NetworkResource{
					{
						Device: "eth0",
						MBits:  512,
					},
				},
			},
			"3.209",
		},
		{
			&structs.Resources{
				CPU:      2048,
				MemoryMB: 500,
				IOPS:     300,
				DiskMB:   4096,
				Networks: []*structs.NetworkResource{
					{
						Device: "eth0",
						MBits:  1024,
					},
				},
			},
			"0.023",
		},
	}

	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			require := require.New(t)
			require.Equal(tc.expectedDistance, fmt.Sprintf("%3.3f", resourceDistance(tc.allocResource, resourceAsk)))
		})

	}

}
