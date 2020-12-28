<?php

namespace Temporal\Tests\Workflow;

use React\Promise\Deferred;
use Temporal\Client\Workflow;
use Temporal\Client\Workflow\WorkflowMethod;

class RuntimeSignalWorkflow
{
    #[WorkflowMethod(name: 'RuntimeSignalWorkflow')]
    public function handler()
    {
        $wait1 = new Deferred();
        $wait2 = new Deferred();

        $counter = 0;

        Workflow::registerSignal('add', function ($value) use (&$counter, $wait1, $wait2) {
            if (is_array($value)) {
                // todo: fix it
                $value = $value[0];
            }

            $counter += $value;
            $wait1->resolve($value);
            $wait2->resolve($value);
        });

        yield $wait1;
        yield $wait2;

        return $counter;
    }
}