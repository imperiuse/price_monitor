package mock_external_api

import (
	"context"
	"fmt"
	"math/rand"
)

func RealWorldExternalApi(_ context.Context) (string, error) {
	// TODO we can simulate context.Deadline, empty response, error in http lib or other... need more context about task
	return genRsp(), nil
}

func genRsp() string {
	// rand.Seed(time.Now().UnixNano()) # not needed I have already executed this in main.go
	const (
		min = 39000
		max = 41000
	)
	return fmt.Sprintf(`{ "amount": %d }`, rand.Intn(max-min)+min)
}
