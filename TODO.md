<p align="center">
  <a href="" rel="noopener">
 <img width=200px height=190px src="https://i.ibb.co/hZdMWvh/optivest-cropped.png" alt="Project logo"></a>
</p>

<h3 align="center">OptiVest</h3>

# TODO 📝

## Features to Implement 🚀

1. **Enhanced Notification System**
   - ~~**Migration to SSE**: Migrate from WebSockets to Server-Sent Events (SSE) for real-time notifications.~~
   - ~~**Redis Integration**: Integrate Redis for efficient notification management.~~
   - ~~**Pending Notifications**: Load pending notifications from Redis and the database while preventing duplication.~~
   - **Notification Endpoints**: 
     -~~ **Status Updates**: Endpoint to update the status of notifications (e.g., mark as read).~~
     - ~~**Deletion**: Endpoint to delete notifications.~~
     -~~ **Fetch Notifications**: Endpoints to get both unread and all notifications for a user.~~

2. **Plan, Payment, and Subscription Capability**
   - **Plan Creation**: Allow users to create different plans (e.g., Basic, Premium, Enterprise).
   - **Payment Integration**: Integrate payment gateways (e.g., Stripe, PayPal) to handle transactions.
   - **Subscription Management**: Enable users to subscribe to plans, manage their subscriptions, and handle renewals and cancellations.

3. **Account Rating System**
   - **Custom Algorithm**: Develop an algorithm to rate accounts based on various factors such as awards obtained, goals completed, and user activity.
   - **Display Ratings**: Show ratings on user profiles and leaderboards.

4. **Educational Feature**
   - **Video Posting**: Allow users to post educational videos.
   - **Comment Functionality**: Enable users to comment on videos.
   - **Like System**: Implement a like system for videos and comments.
   - **Content Moderation**: Add moderation tools to ensure the quality and appropriateness of the content.

5. **Permissions**
   - **User Roles**: Define roles such as regular users, moderators, and admins.
   - **Role-Based Access Control**: Implement permissions based on user roles to control access to different features and administrative functions.

6. **Monthly Report Module**
   - **Report Generation**: Generate monthly reports for subscribed users.
   - **Report Content**: Include statistics such as usage data, goals achieved, and account ratings.
   - **Delivery**: Provide options for users to download reports or receive them via email.

7. **Additional Notification Setups**
   - **Notification Types**: Add various notification types (e.g., email, SMS, in-app notifications).
   - **User Preferences**: Allow users to customize their notification preferences.
   - **Event Triggers**: Set up notifications for different events such as new comments, likes, subscription renewals, and monthly report availability.

8. **Enhance Investment Portfolio Flow**
   - **Include Concurrency on per API Request:** Use concurrency to decrease the time duration of the AI financial analysis. Currrently
                                                 it takes around 30-40s for complete processing. We should get it down to < 10-20s

## Additional Notes

- Maintain high security standards for payment and user data.
- Provide detailed documentation for each new feature.
