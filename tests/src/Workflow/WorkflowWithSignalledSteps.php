<?php


namespace Temporal\Tests\Workflow;

use React\Promise\Deferred;
use React\Promise\PromiseInterface;
use Temporal\Client\Activity\ActivityOptions;
use Temporal\Client\Workflow;
use Temporal\Client\Workflow\WorkflowMethod;
use Temporal\Tests\Activity\SimpleActivity;
use function Symfony\Component\String\s;

class WorkflowWithSignalledSteps
{
    #[WorkflowMethod(name: 'WorkflowWithSignalledSteps')]
    public function handler()
    {
        $simple = Workflow::newActivityStub(
            SimpleActivity::class,
            ActivityOptions::new()->withStartToCloseTimeout(5)
        );

        $value = 0;
        Workflow::registerQuery('value', function () use (&$value) {
            return $value;
        });

        $begin = $this->waitSignal('begin');
        $next1 = $this->waitSignal('next1');
        $next2 = $this->waitSignal('next2');

        yield $begin;
        $value++;

        yield $next1;
        $value++;

        yield $next2;
        $value++;

        return $value;
    }

    // is this correct?
    private function waitSignal(string $name): PromiseInterface
    {
        $signal = new Deferred();
        Workflow::registerSignal($name, function ($value) use ($signal) {
            $signal->resolve($value);
        });

        return $signal->promise();
    }
}