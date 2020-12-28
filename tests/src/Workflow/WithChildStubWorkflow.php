<?php

declare(strict_types=1);

namespace Temporal\Tests\Workflow;

use Temporal\Client\Activity\ActivityOptions;
use Temporal\Client\Internal\Support\DateInterval;
use Temporal\Client\Workflow;
use Temporal\Client\Workflow\WorkflowMethod;
use Temporal\Tests\Activity\SimpleActivity;

class WithChildStubWorkflow
{
    #[WorkflowMethod(name: 'WithChildStubWorkflow')]
    public function handler(string $input): iterable
    {
        $child = Workflow::newChildWorkflowStub(SimpleWorkflow::class);

        return 'Child: ' . (yield $child->handler('child ' . $input));
    }
}
