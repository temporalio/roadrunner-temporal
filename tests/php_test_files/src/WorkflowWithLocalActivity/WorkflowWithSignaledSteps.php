<?php


namespace Temporal\Tests\WorkflowWithLocalActivity;

use React\Promise\Deferred;
use React\Promise\PromiseInterface;
use Temporal\Activity\LocalActivityOptions;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowMethod;
use Temporal\Tests\Activity\SimpleLocalActivity;

#[Workflow\WorkflowInterface]
class WorkflowWithSignaledSteps
{
    #[WorkflowMethod(name: 'WorkflowWithSignaledSteps')]
    public function handler()
    {
        $simple = Workflow::newActivityStub(
            SimpleLocalActivity::class,
            LocalActivityOptions::new()->withStartToCloseTimeout(5)
        );

        $value = 0;
        Workflow::registerQuery('value', function () use (&$value) {
            return $value;
        });

        yield $this->promiseSignal('begin');
        $value++;

        yield $this->promiseSignal('next1');
        $value++;

        yield $this->promiseSignal('next2');
        $value++;

        return $value;
    }

    // is this correct?
    private function promiseSignal(string $name): PromiseInterface
    {
        $signal = new Deferred();
        Workflow::registerSignal($name, function ($value) use ($signal) {
            $signal->resolve($value);
        });

        return $signal->promise();
    }
}
