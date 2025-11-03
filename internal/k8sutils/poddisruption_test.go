package k8sutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type pdbParams struct {
	MinAvailable   *int32
	MaxUnavailable *int32
}

func TestGeneratePodDisruptionBudgetDef_PriorityLogic(t *testing.T) {
	cases := []struct {
		name        string
		params      pdbParams
		clusterSize int32
		expectMin   *int32
		expectMax   *int32
	}{
		{
			name:        "only MinAvailable set",
			params:      pdbParams{MinAvailable: int32Ptr(2)},
			clusterSize: 4,
			expectMin:   int32Ptr(2),
			expectMax:   nil,
		},
		{
			name:        "only MaxUnavailable set",
			params:      pdbParams{MaxUnavailable: int32Ptr(1)},
			clusterSize: 4,
			expectMin:   nil,
			expectMax:   int32Ptr(1),
		},
		{
			name:        "both set, MaxUnavailable wins",
			params:      pdbParams{MinAvailable: int32Ptr(2), MaxUnavailable: int32Ptr(1)},
			clusterSize: 4,
			expectMin:   nil,
			expectMax:   int32Ptr(1),
		},
		{
			name:        "neither set, default quorum",
			params:      pdbParams{},
			clusterSize: 4,
			expectMin:   int32Ptr(3), // (4/2)+1
			expectMax:   nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pdb := generateTestPDB(tc.params, tc.clusterSize)
			if tc.expectMin != nil {
				assert.NotNil(t, pdb.Spec.MinAvailable)
				assert.Equal(t, intstr.Int, pdb.Spec.MinAvailable.Type)
				assert.Equal(t, *tc.expectMin, pdb.Spec.MinAvailable.IntVal)
			} else {
				assert.Nil(t, pdb.Spec.MinAvailable)
			}
			if tc.expectMax != nil {
				assert.NotNil(t, pdb.Spec.MaxUnavailable)
				assert.Equal(t, intstr.Int, pdb.Spec.MaxUnavailable.Type)
				assert.Equal(t, *tc.expectMax, pdb.Spec.MaxUnavailable.IntVal)
			} else {
				assert.Nil(t, pdb.Spec.MaxUnavailable)
			}
		})
	}
}

func int32Ptr(i int32) *int32 { return &i }

func generateTestPDB(params pdbParams, clusterSize int32) *v1.PodDisruptionBudget {
	pdb := &v1.PodDisruptionBudget{Spec: v1.PodDisruptionBudgetSpec{}}
	if params.MaxUnavailable != nil {
		pdb.Spec.MaxUnavailable = &intstr.IntOrString{Type: intstr.Int, IntVal: *params.MaxUnavailable}
	} else if params.MinAvailable != nil {
		pdb.Spec.MinAvailable = &intstr.IntOrString{Type: intstr.Int, IntVal: *params.MinAvailable}
	} else {
		pdb.Spec.MinAvailable = &intstr.IntOrString{Type: intstr.Int, IntVal: (clusterSize / 2) + 1}
	}
	return pdb
}
