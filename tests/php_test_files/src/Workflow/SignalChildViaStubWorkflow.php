<?php

namespace Temporal\Tests\Workflow;

use Temporal\Workflow;
use Temporal\Workflow\WorkflowMethod;

#[Workflow\WorkflowInterface]
class SignalChildViaStubWorkflow
{
    #[WorkflowMethod(name: 'SignalChildViaStubWorkflow')]
    public function handler()
    {
        // typed stub
        $simple = Workflow::newChildWorkflowStub(SimpleSignaledWorkflow::class);

        // start execution
        $call = $simple->handler();

        yield $simple->add(8);

        // expects 8
        return yield $call;
    }
}
