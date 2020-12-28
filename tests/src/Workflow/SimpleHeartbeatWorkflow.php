<?php

declare(strict_types=1);

namespace Temporal\Tests\Workflow;

use Temporal\Client\Activity\ActivityOptions;
use Temporal\Client\Internal\Support\DateInterval;
use Temporal\Client\Workflow;
use Temporal\Client\Workflow\WorkflowMethod;
use Temporal\Tests\Activity\HeartBeatActivity;
use Temporal\Tests\Activity\SimpleActivity;

class SimpleHeartbeatWorkflow
{
    #[WorkflowMethod(name: 'SimpleHeartbeatWorkflow')]
    public function handler(int $iterations): iterable
    {
        $act = Workflow::newActivityStub(
            HeartBeatActivity::class,
            ActivityOptions::new()->withStartToCloseTimeout(50)
        );

        return yield $act->doSomething($iterations);
    }
}
