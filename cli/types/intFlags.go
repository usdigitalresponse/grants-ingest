package types

import "errors"

type TotalsAfter int64

func (t TotalsAfter) Validate() error {
	if t < 0 {
		return errors.New("must be >= 0")
	}
	return nil
}

func (t TotalsAfter) Check(current int64) bool {
	if t <= 0 {
		return false
	}
	return current%int64(t) == 0
}

type ConcurrencyLimit int

func (c ConcurrencyLimit) Validate() error {
	if c <= 0 {
		return errors.New("must be > 0")
	}
	return nil
}
