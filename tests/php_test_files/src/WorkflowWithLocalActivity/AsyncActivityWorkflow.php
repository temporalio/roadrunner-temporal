<?php

declare(strict_types=1);

namespace Temporal\Tests\WorkflowWithLocalActivity;

use Temporal\Activity\ActivityCancellationType;
use Temporal\Activity\LocalActivityOptions;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowMethod;
use Temporal\Tests\Activity\SimpleLocalActivity;

#[Workflow\WorkflowInterface]
class AsyncActivityWorkflow
{
    #[WorkflowMethod(name: 'AsyncActivityWorkflow')]
    public function handler()
    {
        $simple = Workflow::newActivityStub(
            SimpleLocalActivity::class,
            LocalActivityOptions::new()->withStartToCloseTimeout(20)
        );

        return yield $simple->external();
    }
}
