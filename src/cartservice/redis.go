package main

import (
	"github.com/go-redis/redis/v7"
	pb "github.com/triplewy/microservices-demo/src/cartservice/genproto"
)

const (
	CartFieldName = "cart"
	RedisRetryNum = 5
)

type redisClient struct {
	client *redis.Client
}

func newRedisClient(addr string) *redisClient {
	client := redis.NewClient(&redis.Options{
		Addr:       addr,
		Password:   "",
		DB:         0,
		MaxRetries: RedisRetryNum,
	})

	if err := client.Ping().Err(); err != nil {
		panic(err)
	}
	return &redisClient{client: client}
}

func (r *redisClient) AddItem(req *pb.AddItemRequest) error {
	sugar.Infof("AddItem called with userId=%v, productId=%v, quantity=%v", req.GetUserId(), req.GetItem().GetProductId(), req.GetItem().GetQuantity())

	value, err := r.client.HGet(req.GetUserId(), CartFieldName).Result()
	if err != nil && err != redis.Nil {
		return err
	}

	var cart pb.Cart

	if value == "" {
		cart = pb.Cart{
			UserId: req.GetUserId(),
			Items:  nil,
		}
	} else {
		if err := decodeMsgPack([]byte(value), &cart); err != nil {
			return err
		}
	}

	found := false
	for _, item := range cart.Items {
		if item.GetProductId() == req.GetItem().GetProductId() {
			item.Quantity += req.GetItem().GetQuantity()
			found = true
			break
		}
	}

	if !found {
		cart.Items = append(cart.Items, req.GetItem())
	}

	buf, err := encodeMsgPack(cart)
	if err != nil {
		return err
	}
	return r.client.HSet(req.GetUserId(), []string{CartFieldName, buf.String()}).Err()
}

func (r *redisClient) GetCart(userId string) (*pb.Cart, error) {
	sugar.Infof("GetCart called with userId=%v", userId)

	value, err := r.client.HGet(userId, CartFieldName).Result()
	if err != nil && err != redis.Nil {
		sugar.Error(err)
		return nil, err
	}

	if value == "" {
		return &pb.Cart{
			UserId: userId,
			Items:  nil,
		}, nil
	}

	var cart pb.Cart

	if err := decodeMsgPack([]byte(value), &cart); err != nil {
		sugar.Error(err)
		return nil, err
	}

	return &cart, nil
}

func (r *redisClient) EmptyCart(userId string) error {
	sugar.Infof("EmptyCart called with userId=%v", userId)

	return r.client.HSet(userId, []string{CartFieldName, ""}).Err()
}
