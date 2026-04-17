package verdict

type Technical string

const (
	TechnicalNoDiff         Technical = "no_diff"
	TechnicalExpectedDiff   Technical = "expected_diff"
	TechnicalUnexpectedDiff Technical = "unexpected_diff"
	TechnicalSuspiciousDiff Technical = "suspicious_diff"
	TechnicalInconclusive   Technical = "inconclusive"
)

type Recommendation string

const (
	RecommendationGoodToGo    Recommendation = "good_to_go"
	RecommendationNeedsReview Recommendation = "needs_review"
	RecommendationBlock       Recommendation = "block"
	RecommendationRerun       Recommendation = "rerun"
)

func ExitCode(recommendation Recommendation) int {
	switch recommendation {
	case RecommendationGoodToGo:
		return 0
	case RecommendationNeedsReview, RecommendationBlock:
		return 1
	default:
		return 2
	}
}
