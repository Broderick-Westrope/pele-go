package requirement

// styling is what a requirement and an element share: the styles and classes applied to them.
type styling struct {
	cssStyles []string
	classes   []string
}

type requirement struct {
	name          string
	typ           string
	requirementID string
	text          string
	risk          string
	verifyMethod  string
	styling
}

type requirementElement struct {
	name   string
	typ    string
	docRef string
	styling
}

type relation struct {
	typ string
	src string
	dst string
}

type requirementClass struct {
	id         string
	styles     []string
	textStyles []string
}
