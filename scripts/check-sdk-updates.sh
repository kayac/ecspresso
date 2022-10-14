#!/bin/bash

set +e
gh release --repo aws/aws-sdk-go-v2 view | grep service/ecs >> aws-sdk-go-v2-ecs.md
uniq aws-sdk-go-v2-ecs.md > aws-sdk-go-v2-ecs.md.tmp
mv aws-sdk-go-v2-ecs.md.tmp aws-sdk-go-v2-ecs.md
git diff --exit-code aws-sdk-go-v2-ecs.md
if [[ $? -eq 0 ]]; then
	echo "No changes detected"
	exit 0
fi

BRANCH="aws-sdk-go-v2-ecs-$(date +%Y-%m-%d)"
git config --local user.email "fujiwara.shunichiro@gmail.com"
git config --local user.name "GitHub Actions"
git switch -c $BRANCH
git add aws-sdk-go-v2-ecs.md
git commit -m "Update aws-sdk-go-v2 about service/ecs $(date +%Y-%m-%d)"
git push origin $BRANCH
gh pr create --title "Update aws-sdk-go-v2 about service/ecs $(date +%Y-%m-%d)" --fill
