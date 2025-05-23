package cgroups

import (
	"go.uber.org/zap"
)

// Sometimes we get into races with cgroups creation and deletion during observing cgroups hierarchy. Simple retries
// with some small delay won't help because for example during user session opening systemd may configure groups for
// seconds. So we use different approach here and keep information about races between collections.
type RaceController struct {
	logger *zap.SugaredLogger

	maxRetries     int
	maxActiveRaces int

	current map[string]struct{}
	active  map[string]int
}

func NewRaceController(logger *zap.SugaredLogger, maxRetries int, maxActiveRaces int) *RaceController {
	return &RaceController{
		logger: logger,

		maxRetries:     maxRetries,
		maxActiveRaces: maxActiveRaces,

		current: make(map[string]struct{}),
		active:  make(map[string]int),
	}
}

func (c *RaceController) OnCollectionStarted() {
	clear(c.current)
}

// FIXME(konishchev): Debug logging?
func (c *RaceController) Check(group *Group, err error) error {
	races := c.active[group.Name]

	if _, ok := c.current[group.Name]; !ok {
		races += 1
		c.current[group.Name] = struct{}{}
		c.active[group.Name] = races
	}

	if active := len(c.active); active > c.maxActiveRaces {
		c.logger.Warnf("Race detector: too many cgroups with races: %d.", active)
		return err
	} else if races > c.maxRetries {
		c.logger.Warnf("Race detector: too many races on %q group (%d).", group.Name, races)
		return err
	}

	c.logger.Warnf("Suppressing a possible race on %q cgroup (%d): %s.", group.Name, races, err)
	return nil
}

func (c *RaceController) OnCollectionFinished() {
	for name := range c.active {
		if _, ok := c.current[name]; !ok {
			delete(c.active, name)
		}
	}
	clear(c.current)
}
