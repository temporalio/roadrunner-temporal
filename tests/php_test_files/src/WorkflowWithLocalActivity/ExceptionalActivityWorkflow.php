<?php

namespace Temporal\Tests\WorkflowWithLocalActivity;

use Temporal\Activity\LocalActivityOptions;
use Temporal\Common\RetryOptions;
use Temporal\Tests\Activity\SimpleLocalActivity;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowMethod;

#[Workflow\WorkflowInterface]
class ExceptionalActivityWorkflow
{
    #[WorkflowMethod(name: 'ExceptionalActivityWorkflow')]
    public function handler()
    {
        $simple = Workflow::newActivityStub(
            SimpleLocalActivity::class,
            LocalActivityOptions::new()->withStartToCloseTimeout(5)
                ->withRetryOptions((new RetryOptions())->withMaximumAttempts(1))
        );

        return yield $simple->fail();
    }
}
