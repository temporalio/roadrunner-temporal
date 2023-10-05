<?php

namespace Temporal\Tests\WorkflowWithLocalActivity;

use Temporal\Activity\LocalActivityOptions;
use Temporal\Common\Uuid;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowMethod;
use Temporal\Tests\Activity\SimpleLocalActivity;

#[Workflow\WorkflowInterface]
class SideEffectWorkflow
{
    #[WorkflowMethod(name: 'SideEffectWorkflow')]
    public function handler(string $input): iterable
    {
        $simple = Workflow::newActivityStub(
            SimpleLocalActivity::class,
            LocalActivityOptions::new()->withStartToCloseTimeout(5)
        );

        $result = yield Workflow::sideEffect(
            function () use ($input) {
                return $input . '-42';
            }
        );

        return yield $simple->lower($result);
    }
}
