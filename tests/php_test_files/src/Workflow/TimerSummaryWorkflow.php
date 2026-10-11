<?php

declare(strict_types=1);

namespace Temporal\Tests\Workflow;

use Temporal\Activity\ActivityOptions;
use Temporal\Workflow;
use Temporal\Workflow\TimerOptions;
use Temporal\Workflow\WorkflowMethod;
use Temporal\Tests\Activity\SimpleActivity;

#[Workflow\WorkflowInterface]
class TimerSummaryWorkflow
{
    #[WorkflowMethod(name: 'TimerSummaryWorkflow')]
    public function handler(string $input): iterable
    {
        $simple = Workflow::newActivityStub(
            SimpleActivity::class,
            ActivityOptions::new()->withStartToCloseTimeout(5)
        );

        yield Workflow::timer(1, TimerOptions::new()->withSummary('rr-timer-summary'));

        return yield $simple->lower($input);
    }
}
