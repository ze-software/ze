// Design: docs/labs/l2tp-interop.md -- 3 x 16 independent PPP identities.
// Related: scale_inputs_test.go -- checks the actual mounted peer inputs.
package l2tp

import "fmt"

const scaleSessionsPerLAC = 16

func scaleUsername(lac, session int) string {
	return fmt.Sprintf("scale-%d-%02d", lac, session)
}

func scaleInterface(lac, session int) string {
	return fmt.Sprintf("sc%ds%02d", lac, session)
}

func scaleUserLocation(user string) (int, int, error) {
	for lac := 1; lac <= scaleLACCount; lac++ {
		for session := 1; session <= scaleSessionsPerLAC; session++ {
			if scaleUsername(lac, session) == user {
				return lac, session, nil
			}
		}
	}
	return 0, 0, fmt.Errorf("unknown scale subscriber %q", user)
}

func scaleUsers(lacs int) []string {
	users := make([]string, 0, lacs*scaleSessionsPerLAC)
	for lac := 1; lac <= lacs; lac++ {
		for session := 1; session <= scaleSessionsPerLAC; session++ {
			users = append(users, scaleUsername(lac, session))
		}
	}
	return users
}
