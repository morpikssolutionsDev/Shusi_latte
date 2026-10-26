package dbx

import (
	"io/fs"

	"sushilatte/api/db"
)

func mustSub() fs.FS {
	sub, err := fs.Sub(db.Migrations, "migrations")
	if err != nil {
		panic(err)
	}
	return sub
}
