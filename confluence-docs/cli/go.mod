module github.com/lybel-app/skills/confluence-docs/cli

go 1.26.2

require (
	github.com/lybel-app/skills/pkg/atlassian v0.0.0-00010101000000-000000000000
	github.com/yuin/goldmark v1.7.8
)

replace github.com/lybel-app/skills/pkg/atlassian => ../../pkg/atlassian
