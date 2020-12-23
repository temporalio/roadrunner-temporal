<?php


namespace Temporal\Tests\Workflow;

use React\Promise\Deferred;
use React\Promise\PromiseInterface;
use Temporal\Client\Activity\ActivityOptions;
use Temporal\Client\Workflow;
use Temporal\Client\Workflow\WorkflowMethod;
use Temporal\Tests\Activity\SimpleActivity;
use function Symfony\Component\String\s;

class WorkflowWithSequence
{
    #[WorkflowMethod(name: 'WorkflowWithSequence')]
    public function handler()
    {
        $simple = Workflow::newActivityStub(
            SimpleActivity::class,
            ActivityOptions::new()->withStartToCloseTimeout(5)
        );

        $a = $simple->echo('a');
        $b = $simple->echo('b');

        yield $a;
        yield $b;

        return 'OK';
    }
}