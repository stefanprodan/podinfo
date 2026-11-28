// Package environ returns the process environment and request headers with
// credentials masked, for the endpoints that echo them back.
//
// A value is masked when the name of its variable or header marks it as a
// credential (PASSWORD, TOKEN, KEY, Authorization, Cookie and the like), and
// the secrets embedded in URL values are masked in place so the rest of the
// URL stays readable.
package environ
