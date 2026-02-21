package store

import sq "github.com/Masterminds/squirrel"

func psql() sq.StatementBuilderType {
	return sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
}
