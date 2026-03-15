package server

func fundingPercentFromCents(amountRaisedCents, amountNeededCents int) int {
	if amountNeededCents <= 0 {
		return 0
	}

	fundingPercent := (amountRaisedCents * 100) / amountNeededCents
	if fundingPercent < 0 {
		fundingPercent = 0
	}
	if fundingPercent > 100 {
		fundingPercent = 100
	}
	return fundingPercent
}
