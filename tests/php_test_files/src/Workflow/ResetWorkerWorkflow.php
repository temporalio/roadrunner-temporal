<?php

namespace Temporal\Tests\Workflow;

use Temporal\DataConverter\Type;
use Temporal\Workflow;
use Temporal\Workflow\ReturnType;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;

#[WorkflowInterface]
final class ResetWorkerWorkflow
{
    #[WorkflowMethod('ResetWorkerWorkflow')]
    #[ReturnType(Type::TYPE_STRING)]
    public function expire(int $seconds = 10): \Generator
    {
        yield Workflow::timer($seconds);

        return yield 'Timer';
    }

    /**
     * Kill the PHP Worker with delay.
     *
     * @param int<1, max> $sleep Freeze the worker until killed.
     */
    #[Workflow\QueryMethod('die')]
    public function die(int $sleep = 4)
    {
        sleep($sleep);
        exit(1);
    }
}
