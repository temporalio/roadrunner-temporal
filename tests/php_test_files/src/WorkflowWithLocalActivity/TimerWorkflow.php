<?php

declare(strict_types=1);

namespace Temporal\Tests\WorkflowWithLocalActivity;

use Temporal\Activity\LocalActivityOptions;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowMethod;
use Temporal\Tests\Activity\SimpleLocalActivity;

#[Workflow\WorkflowInterface]
class TimerWorkflow
{
    #[WorkflowMethod(name: 'TimerWorkflow')]
    public function handler(string $input): iterable
    {
        $simple = Workflow::newActivityStub(
            SimpleLocalActivity::class,
            LocalActivityOptions::new()->withStartToCloseTimeout(5)
        );

        yield Workflow::timer(3);

        return yield $simple->lower($input);
    }
}
