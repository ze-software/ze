package errorlint

// Option configures an analyzer created by NewAnalyzer.
type Option func(*config)

// config holds the settings of a single analyzer, so that multiple analyzers can coexist without
// affecting each other.
type config struct {
	checkComparison  bool
	checkAsserts     bool
	checkErrorf      bool
	checkErrorfMulti bool

	allowed *allowList
}

func WithAllowedErrors(ap []AllowPair) Option {
	return func(c *config) {
		c.allowed.addErrors(ap)
	}
}

func WithAllowedWildcard(ap []AllowPair) Option {
	return func(c *config) {
		c.allowed.addWildcards(ap)
	}
}

func WithComparison(enabled bool) Option {
	return func(c *config) {
		c.checkComparison = enabled
	}
}

func WithAsserts(enabled bool) Option {
	return func(c *config) {
		c.checkAsserts = enabled
	}
}
