package client

import (
	"context"
	"fmt"
	"log"

	pb "api-gateway/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type NotificationClient struct {
	client pb.NotificationServiceClient
	conn   *grpc.ClientConn
}

func NewNotificationClient(host, port string) (*NotificationClient, error) {
	address := fmt.Sprintf("%s:%s", host, port)

	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create notification client for %s: %w", address, err)
	}

	log.Printf("Notification Service client initialized for %s", address)

	return &NotificationClient{
		client: pb.NewNotificationServiceClient(conn),
		conn:   conn,
	}, nil
}

func (c *NotificationClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *NotificationClient) GetNotifications(ctx context.Context, userID string, limit, offset int32) (*pb.GetNotificationsResponse, error) {
	return c.client.GetNotifications(ctx, &pb.GetNotificationsRequest{
		UserId: userID,
		Limit:  limit,
		Offset: offset,
	})
}

func (c *NotificationClient) MarkAsRead(ctx context.Context, notificationID, userID string) (*pb.MarkAsReadResponse, error) {
	return c.client.MarkAsRead(ctx, &pb.MarkAsReadRequest{
		NotificationId: notificationID,
		UserId:         userID,
	})
}

func (c *NotificationClient) DeleteNotification(ctx context.Context, notificationID, userID string) (*pb.DeleteNotificationResponse, error) {
	return c.client.DeleteNotification(ctx, &pb.DeleteNotificationRequest{
		NotificationId: notificationID,
		UserId:         userID,
	})
}