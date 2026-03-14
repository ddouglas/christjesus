package types

import "fmt"

var (
	ErrNeedNotFound       = fmt.Errorf("need not found")
	ErrNeedAlreadyDeleted = fmt.Errorf("need already deleted")
	ErrNeedNotDeleted     = fmt.Errorf("need not deleted")
	ErrUserNotFound       = fmt.Errorf("user not found")
)
