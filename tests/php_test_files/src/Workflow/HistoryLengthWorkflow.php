<?php

declare(strict_types=1);

namespace Temporal\Tests\Workflow;

use Temporal\Activity\ActivityOptions;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowMethod;
use Temporal\Tests\Activity\SimpleActivity;

#[Workflow\WorkflowInterface]
class HistoryLengthWorkflow
{
    #[WorkflowMethod(name: 'HistoryLengthWorkflow')]
    public function handler(): iterable
    {
        $result = [];
        $result[] = Workflow::getInfo()->historyLength;

        $simple = Workflow::newActivityStub(
            SimpleActivity::class,
            ActivityOptions::new()->withStartToCloseTimeout(5)
        );

        yield Workflow::timer(1);
        $result[] = Workflow::getInfo()->historyLength;

        yield Workflow::sideEffect(static fn() => '-42');
        $result[] = Workflow::getInfo()->historyLength;

        yield $simple->lower('BAR');
        $result[] = Workflow::getInfo()->historyLength;

        return $result;
    }
}
