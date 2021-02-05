.PHONY: test create deploy rollback refresh deploy-no-update-service scale down delete help run-task status

export CLUSTER := ecspresso-test
export SERVICE := nginx-$(CIRCLE_BRANCH)
CONFIG ?= config.yaml

help:

test: verify create run-task deploy rollback refresh deploy-no-update-service scale down delete

status:
	ecspresso --config $(CONFIG) status --envfile envfile --events 10 --debug

create:
	ecspresso --config $(CONFIG) --envfile envfile create --dry-run
	ecspresso --config $(CONFIG) --envfile envfile create --debug

register:
	ecspresso --config $(CONFIG) --envfile envfile register --dry-run
	ecspresso --config $(CONFIG) --envfile envfile register --debug

deploy:
	ecspresso --config $(CONFIG) --envfile envfile deploy --dry-run
	ecspresso --config $(CONFIG) --envfile envfile deploy --debug

deploy-no-update-service:
	ecspresso --config $(CONFIG) --envfile envfile deploy --no-update-service --dry-run
	ecspresso --config $(CONFIG) --envfile envfile deploy --no-update-service --force-new-deployment --debug

refresh:
	ecspresso --config $(CONFIG) --envfile envfile refresh --dry-run
	ecspresso --config $(CONFIG) --envfile envfile refresh --debug

rollback:
	ecspresso --config $(CONFIG) --envfile envfile rollback --deregister-task-definition --dry-run
	ecspresso --config $(CONFIG) --envfile envfile rollback --deregister-task-definition --debug

scale:
	ecspresso --config $(CONFIG) --envfile envfile scale --tasks 2 --dry-run
	ecspresso --config $(CONFIG) --envfile envfile scale --tasks 2 --debug

down:
	ecspresso --config $(CONFIG) --envfile envfile scale --tasks 0 --dry-run
	ecspresso --config $(CONFIG) --envfile envfile scale --tasks 0 --debug

delete:
	ecspresso --config $(CONFIG) --envfile envfile delete --dry-run
	ecspresso --config $(CONFIG) --envfile envfile delete --force --debug

run-task:
	ecspresso --config $(CONFIG) --envfile envfile run \
	--overrides '{"containerOverrides":[{"name":"nginx", "command":["nginx", "-V"]}]}'

wait:
	ecspresso --config $(CONFIG) --envfile envfile wait

verify:
	ecspresso --config $(CONFIG) --envfile envfile verify --debug

diff:
	ecspresso --config $(CONFIG) --envfile envfile diff
