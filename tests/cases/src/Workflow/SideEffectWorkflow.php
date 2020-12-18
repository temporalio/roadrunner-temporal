<?php

namespace Temporal\Tests\Workflow;

use Temporal\Client\Activity\ActivityOptions;
use Temporal\Client\Common\Uuid;
use Temporal\Client\Workflow;
use Temporal\Client\Workflow\WorkflowMethod;
use Temporal\Tests\Activity\SimpleActivity;

class SideEffectWorkflow
{
    #[WorkflowMethod(name: 'SideEffectWorkflow')]
    public function handler(string $input): iterable
    {
        $simple = Workflow::newActivityStub(
            SimpleActivity::class,
            ActivityOptions::new()->withStartToCloseTimeout(5)
        );

        $result = yield Workflow::sideEffect(function () use ($input) {
            return $input . '-' . Uuid::v4();
        });

        return yield $simple->lower($result);
    }
}