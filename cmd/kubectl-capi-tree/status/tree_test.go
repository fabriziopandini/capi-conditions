package status

import (
	"testing"

	. "github.com/onsi/gomega"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/cluster-api/util/conditions"
)

func Test_hasSameReadyStatusSeverityAndReason(t *testing.T) {
	readyTrue := conditions.TrueCondition(clusterv1.ReadyCondition)
	readyFalseReasonInfo := conditions.FalseCondition(clusterv1.ReadyCondition, "Reason", clusterv1.ConditionSeverityInfo, "message falseInfo1")
	readyFalseAnotherReasonInfo := conditions.FalseCondition(clusterv1.ReadyCondition, "AnotherReason", clusterv1.ConditionSeverityInfo, "message falseInfo1")
	readyFalseReasonWarning := conditions.FalseCondition(clusterv1.ReadyCondition, "Reason", clusterv1.ConditionSeverityWarning, "message falseInfo1")

	type args struct {
		a *clusterv1.Condition
		b *clusterv1.Condition
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Objects without conditions should group",
			args: args{
				a: nil,
				b: nil,
			},
			want: true,
		},
		{
			name: "Objects with same Ready condition",
			args: args{
				a: readyTrue,
				b: readyTrue,
			},
			want: true,
		},
		{
			name: "Objects with different Ready.Status",
			args: args{
				a: readyTrue,
				b: readyFalseReasonInfo,
			},
			want: false,
		},
		{
			name: "Objects with different Ready.Reason",
			args: args{
				a: readyFalseReasonInfo,
				b: readyFalseAnotherReasonInfo,
			},
			want: false,
		},
		{
			name: "Objects with different Ready.Severity",
			args: args{
				a: readyFalseReasonInfo,
				b: readyFalseReasonWarning,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			got := hasSameReadyStatusSeverityAndReason(tt.args.a, tt.args.b)
			g.Expect(got).To(Equal(tt.want))
		})
	}
}
