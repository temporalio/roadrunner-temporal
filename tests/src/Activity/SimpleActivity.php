<?php

namespace Temporal\Tests\Activity;

use Temporal\Client\Activity\ActivityInterface;
use Temporal\Client\Activity\ActivityMethod;
use Temporal\Tests\DTO\Message;
use Temporal\Tests\DTO\User;

#[ActivityInterface(prefix: "SimpleActivity.")]
class SimpleActivity
{
    #[ActivityMethod]
    public function echo(string $input): string
    {
        return strtoupper($input);
    }

    #[ActivityMethod]
    public function lower(string $input): string
    {
        return strtolower($input);
    }

    #[ActivityMethod]
    public function greet(User $user): Message
    {
        return new Message(sprintf("Hello %s <%s>", $user->name, $user->email));
    }
}