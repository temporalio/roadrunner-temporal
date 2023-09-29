<?php

declare(strict_types=1);

namespace Temporal\Tests\WorkflowWithLocalActivity;

use Temporal\Activity\LocalActivityOptions;
use Temporal\Promise;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowMethod;
use Temporal\Tests\Activity\SimpleLocalActivity;

#[Workflow\WorkflowInterface]
class ParallelScopesWorkflow
{
    #[WorkflowMethod(name: 'ParallelScopesWorkflow')]
    public function handler(string $input)
    {
        $simple = Workflow::newActivityStub(
            SimpleLocalActivity::class,
            LocalActivityOptions::new()->withStartToCloseTimeout(5)
        );

        $a = Workflow::async(function () use ($simple, $input) {
            return yield $simple->echo($input);
        });

        $b = Workflow::async(function () use ($simple, $input) {
            return yield $simple->lower($input);
        });

        [$ra, $rb] = yield Promise::all([$a, $b]);

        return sprintf('%s|%s|%s', $ra, $input, $rb);
    }
}
