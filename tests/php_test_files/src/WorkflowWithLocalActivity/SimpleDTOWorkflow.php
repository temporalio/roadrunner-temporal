<?php

declare(strict_types=1);

namespace Temporal\Tests\WorkflowWithLocalActivity;

use Temporal\Activity\LocalActivityOptions;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowMethod;
use Temporal\Tests\Activity\SimpleLocalActivity;
use Temporal\Tests\DTO\Message;
use Temporal\Tests\DTO\User;

#[Workflow\WorkflowInterface]
class SimpleDTOWorkflow
{
    #[WorkflowMethod(name: 'SimpleDTOWorkflow')]//, returnType: Message::class)]
    public function handler(
        User $user
    ) {
        $simple = Workflow::newActivityStub(
            SimpleLocalActivity::class,
            LocalActivityOptions::new()
                ->withStartToCloseTimeout(5)
        );

        $value = yield $simple->greet($user);

        if (!$value instanceof Message) {
            return "FAIL";
        }

        return $value;
    }
}
