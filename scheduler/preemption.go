package scheduler

import (
	"fmt"
	"math"
	"sort"

	"github.com/hashicorp/nomad/nomad/structs"
)

// resourceDistance returns how close the resource is to the resource being asked for
// It is calculated by first computing a relative fraction and then measuring how close
// that is to zero. Lower values are closer
func resourceDistance(resource *structs.Resources, resourceAsk *structs.Resources) float64 {
	memoryCoord, cpuCoord, iopsCoord, diskMBCoord, mbitsCoord := 0.0, 0.0, 0.0, 0.0, 0.0
	if resourceAsk.MemoryMB > 0 {
		memoryCoord = float64(resourceAsk.MemoryMB-resource.MemoryMB) / float64(resourceAsk.MemoryMB)
	}
	if resourceAsk.CPU > 0 {
		cpuCoord = float64(resourceAsk.CPU-resource.CPU) / float64(resourceAsk.CPU)
	}
	if resourceAsk.IOPS > 0 {
		iopsCoord = float64(resourceAsk.IOPS-resource.IOPS) / float64(resourceAsk.IOPS)
	}
	if resourceAsk.DiskMB > 0 {
		diskMBCoord = float64(resourceAsk.DiskMB-resource.DiskMB) / float64(resourceAsk.DiskMB)
	}

	// TODO(preetha): implement this correctly
	if len(resourceAsk.Networks) > 0 && len(resource.Networks) > 0 {
		mbitsCoord = float64(resourceAsk.Networks[0].MBits-resource.Networks[0].MBits) / float64(resourceAsk.Networks[0].MBits)
	}

	originDist := math.Sqrt(
		math.Pow(memoryCoord, 2) +
			math.Pow(cpuCoord, 2) +
			math.Pow(iopsCoord, 2) +
			math.Pow(mbitsCoord, 2) +
			math.Pow(diskMBCoord, 2))
	return originDist
}

// GetPreemptibleAllocs computes a list of allocations to preempt to accommodate
// the resource asked for. Only allocs with a job priority < 10 of jobPriority are considered
// This currently does not account for static port asks
func GetPreemptibleAllocs(jobPriority int, current []*structs.Allocation, resourceAsk *structs.Resources) []*structs.Allocation {

	groupedAllocs := filterAndGroupPreemptibleAllocs(jobPriority, current)

	var bestAllocs []*structs.Allocation
	requirementsMet := false
	var preemptedResources *structs.Resources
	for _, allocGrp := range groupedAllocs {
		for len(allocGrp.allocs) > 0 && !requirementsMet {
			closestAllocIndex := -1
			bestDistance := math.MaxFloat64
			// find the alloc with the closest distance
			for index, alloc := range allocGrp.allocs {
				distance := resourceDistance(alloc.Resources, resourceAsk)
				fmt.Printf("%+v, %3.3f\n", alloc.Resources, distance)
				if distance < bestDistance {
					bestDistance = distance
					closestAllocIndex = index
				}
			}
			if closestAllocIndex == -1 {
				// This means no option in the current list was eligible for preemption
				// Can stop looking
				break
			}
			closestAlloc := allocGrp.allocs[closestAllocIndex]
			if preemptedResources == nil {
				preemptedResources = closestAlloc.Resources.Copy()
			} else {
				preemptedResources.Add(closestAlloc.Resources)
			}
			requirementsMet = MeetsRequirements(preemptedResources, resourceAsk)
			bestAllocs = append(bestAllocs, closestAlloc)
			allocGrp.allocs[closestAllocIndex] = allocGrp.allocs[len(allocGrp.allocs)-1]
			allocGrp.allocs = allocGrp.allocs[:len(allocGrp.allocs)-1]
		}
		if requirementsMet {
			break
		}
	}

	// Early return if all allocs examined and requirements were not met
	if !requirementsMet {
		return nil
	}

	// We do another pass to eliminate unnecessary preemptions
	// This filters out allocs whose resources are already covered by another alloc

	// Sort by distance reversed to surface any superset allocs first
	sort.Slice(bestAllocs, func(i, j int) bool {
		distance1 := resourceDistance(bestAllocs[i].Resources, resourceAsk)
		distance2 := resourceDistance(bestAllocs[j].Resources, resourceAsk)
		return distance1 > distance2
	})

	var filteredBestAllocs []*structs.Allocation
	// Reset aggregate preempted resources so that we can do another pass
	preemptedResources = nil
	for _, alloc := range bestAllocs {
		if preemptedResources == nil {
			preemptedResources = alloc.Resources
		} else {
			preemptedResources.Add(alloc.Resources)
		}
		filteredBestAllocs = append(filteredBestAllocs, alloc)
		requirementsMet := MeetsRequirements(preemptedResources, resourceAsk)
		if requirementsMet {
			break
		}
	}

	return filteredBestAllocs

}

// MeetsRequirements checks if the first resource meets or exceeds the second resource's requirements
// TODO network iops is pretty broken and needs to be rewritten
func MeetsRequirements(first *structs.Resources, second *structs.Resources) bool {
	if first.CPU < second.CPU {
		return false
	}
	if first.MemoryMB < second.MemoryMB {
		return false
	}
	if first.DiskMB < second.DiskMB {
		return false
	}
	if first.IOPS < second.IOPS {
		return false
	}
	if len(first.Networks) > 0 && len(second.Networks) > 0 {
		if first.Networks[0].MBits < second.Networks[0].MBits {
			return false
		}
	}
	return true
}

type groupedAllocs struct {
	priority int
	allocs   []*structs.Allocation
}

func filterAndGroupPreemptibleAllocs(jobPriority int, current []*structs.Allocation) []*groupedAllocs {
	allocsByPriority := make(map[int][]*structs.Allocation)
	for _, alloc := range current {
		// Skip ineligible allocs
		if alloc.Job.Priority >= jobPriority+10 {
			continue
		}
		grpAllocs, ok := allocsByPriority[alloc.Job.Priority]
		if !ok {
			grpAllocs = make([]*structs.Allocation, 0)
		}
		grpAllocs = append(grpAllocs, alloc)
		allocsByPriority[alloc.Job.Priority] = grpAllocs
	}

	var groupedSortedAllocs []*groupedAllocs
	for priority, allocs := range allocsByPriority {
		groupedSortedAllocs = append(groupedSortedAllocs, &groupedAllocs{
			priority: priority,
			allocs:   allocs})
	}

	sort.Slice(groupedSortedAllocs, func(i, j int) bool {
		return groupedSortedAllocs[i].priority < groupedSortedAllocs[j].priority
	})

	// Sort by priority
	return groupedSortedAllocs
}
