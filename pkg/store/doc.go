// Package store keeps content-addressed files in a directory, each named by
// the SHA1 hash of its content.
//
// Write stores the content under a temporary name and renames it into place,
// so a reader never sees a partial file; Sweep removes the temporary files a
// crash left behind. Read rejects anything that is not a hash, which keeps
// paths inside the directory. Both run in a span of their own.
package store
