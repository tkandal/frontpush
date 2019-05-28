package frontpush

import "io"

/*
 * Copyright (c) 2019 Norwegian University of Science and Technology
 */

// Pusher is an interface that defines a pusher
type Pusher interface {
	// Push pushes cards to destination endpoint
	Push(io.Reader) (io.Reader, error)
}
