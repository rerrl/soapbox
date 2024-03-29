package users

import (
	"context"
	"database/sql"
	"strings"

	"github.com/soapboxsocial/soapbox/pkg/users/types"
)

// SearchUser is used for our search engine.
type SearchUser struct {
	ID          int    `json:"id"`
	DisplayName string `json:"display_name"`
	Username    string `json:"username"`
	Image       string `json:"image"`
	Bio         string `json:"bio"`
	Followers   int    `json:"followers"`
	RoomTime    int    `json:"room_time"`
}

type LinkedAccount struct {
	ID       uint64 `json:"id"`
	Provider string `json:"provider"`
	Username string `json:"username"`
}

// Profile represents the User for public profile usage.
// This means certain fields like `email` are omitted,
// and others are added like `follower_counts` and relationships.
type Profile struct {
	ID             int             `json:"id"`
	DisplayName    string          `json:"display_name"`
	Username       string          `json:"username"`
	Bio            string          `json:"bio"`
	Followers      int             `json:"followers"`
	Following      int             `json:"following"`
	FollowedBy     *bool           `json:"followed_by,omitempty"`
	IsFollowing    *bool           `json:"is_following,omitempty"`
	IsBlocked      *bool           `json:"is_blocked,omitempty"`
	Image          string          `json:"image"`
	LinkedAccounts []LinkedAccount `json:"linked_accounts"`
}

type NotificationUser struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Image    string `json:"image"`
}

type Backend struct {
	db *sql.DB
}

func NewBackend(db *sql.DB) *Backend {
	return &Backend{
		db: db,
	}
}

func (b *Backend) GetIDForUsername(username string) (int, error) {
	stmt, err := b.db.Prepare("SELECT id FROM users WHERE username = $1;")
	if err != nil {
		return 0, err
	}

	var id int
	err = stmt.QueryRow(username).Scan(&id)
	if err != nil {
		return 0, err
	}

	return id, nil
}

func (b *Backend) GetUserByUsername(username string) (*types.User, error) {
	stmt, err := b.db.Prepare("SELECT id, display_name, image, bio FROM users WHERE username = $1;")
	if err != nil {
		return nil, err
	}

	user := &types.User{}
	err = stmt.QueryRow(username).Scan(&user.ID, &user.DisplayName, &user.Image, &user.Bio)
	if err != nil {
		return nil, err
	}

	user.Username = username

	return user, nil
}

func (b *Backend) GetUserForSearchEngine(id int) (*SearchUser, error) {
	query := `SELECT 
       id, display_name, username, image, bio,
       (SELECT COUNT(*) FROM followers WHERE user_id = id) AS followers, 
       (SELECT CAST(FLOOR(SUM(EXTRACT(EPOCH FROM (left_time - join_time)))) as INT) FROM user_room_logs WHERE user_id = id AND join_time >= NOW() - INTERVAL '7 DAYS' AND visibility = 'public') FROM users WHERE id = $1;`

	stmt, err := b.db.Prepare(query)
	if err != nil {
		return nil, err
	}

	roomTime := sql.NullInt64{}

	profile := &SearchUser{}
	err = stmt.QueryRow(id).Scan(
		&profile.ID,
		&profile.DisplayName,
		&profile.Username,
		&profile.Image,
		&profile.Bio,
		&profile.Followers,
		&roomTime,
	)

	if roomTime.Valid {
		profile.RoomTime = int(roomTime.Int64)
	} else {
		profile.RoomTime = 0
	}

	if err != nil {
		return nil, err
	}

	return profile, nil
}

func (b *Backend) GetMyProfile(id int) (*Profile, error) {
	query := `SELECT 
       id, display_name, username, image, bio,
       (SELECT COUNT(*) FROM followers WHERE user_id = id) AS followers,
       (SELECT COUNT(*) FROM followers WHERE follower = id) AS following FROM users WHERE id = $1;`

	stmt, err := b.db.Prepare(query)
	if err != nil {
		return nil, err
	}

	profile := &Profile{}
	err = stmt.QueryRow(id).Scan(
		&profile.ID,
		&profile.DisplayName,
		&profile.Username,
		&profile.Image,
		&profile.Bio,
		&profile.Followers,
		&profile.Following,
	)
	if err != nil {
		return nil, err
	}

	accounts, err := b.LinkedAccounts(id)
	if err == nil {
		profile.LinkedAccounts = accounts
	}

	return profile, nil
}

func (b *Backend) ProfileByID(id, from int) (*Profile, error) {
	query := `SELECT 
       id, display_name, username, image, bio,
       (SELECT COUNT(*) FROM followers WHERE user_id = id) AS followers,
       (SELECT COUNT(*) FROM followers WHERE follower = id) AS following,
       (SELECT COUNT(*) FROM followers WHERE follower = id AND user_id = $1) AS followed_by,
       (SELECT COUNT(*) FROM followers WHERE follower = $1 AND user_id = id) AS is_following,
       (SELECT COUNT(*) FROM blocks WHERE user_id = $1 AND blocked = id) AS is_following FROM users WHERE id = $2;`

	stmt, err := b.db.Prepare(query)
	if err != nil {
		return nil, err
	}

	profile := &Profile{}

	var followedBy, isFollowing, isBlocked int
	err = stmt.QueryRow(from, id).Scan(
		&profile.ID,
		&profile.DisplayName,
		&profile.Username,
		&profile.Image,
		&profile.Bio,
		&profile.Followers,
		&profile.Following,
		&followedBy,
		&isFollowing,
		&isBlocked,
	)

	if err != nil {
		return nil, err
	}

	following := isFollowing == 1
	followed := followedBy == 1
	blocked := isBlocked == 1
	profile.IsFollowing = &following
	profile.FollowedBy = &followed
	profile.IsBlocked = &blocked

	accounts, err := b.LinkedAccounts(id)
	if err == nil {
		profile.LinkedAccounts = accounts
	}

	return profile, nil
}

func (b *Backend) NotificationUserFor(id int) (*NotificationUser, error) {
	query := `SELECT id, username, image FROM users WHERE id = $1;`

	stmt, err := b.db.Prepare(query)
	if err != nil {
		return nil, err
	}

	profile := &NotificationUser{}

	err = stmt.QueryRow(id).Scan(
		&profile.ID,
		&profile.Username,
		&profile.Image,
	)

	if err != nil {
		return nil, err
	}

	return profile, nil
}

func (b *Backend) IsAppleIDAccount(email string) (bool, error) {
	stmt, err := b.db.Prepare("SELECT COUNT(*) FROM apple_authentication WHERE user_id = (SELECT id FROM users WHERE email = $1);")
	if err != nil {
		return false, err
	}

	var id int
	err = stmt.QueryRow(email).Scan(&id)
	if err != nil {
		return false, err
	}

	return id == 1, nil
}

func (b *Backend) FindByAppleID(id string) (*types.User, error) {
	stmt, err := b.db.Prepare("SELECT id, display_name, username, image, bio, email FROM users INNER JOIN apple_authentication ON users.id = apple_authentication.user_id WHERE apple_authentication.apple_user = $1;")
	if err != nil {
		return nil, err
	}

	user := &types.User{}
	err = stmt.QueryRow(id).Scan(&user.ID, &user.DisplayName, &user.Username, &user.Image, &user.Bio, &user.Email)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (b *Backend) FindByID(id int) (*types.User, error) {
	stmt, err := b.db.Prepare("SELECT id, display_name, username, image, bio, email FROM users WHERE id = $1;")
	if err != nil {
		return nil, err
	}

	user := &types.User{}
	err = stmt.QueryRow(id).Scan(&user.ID, &user.DisplayName, &user.Username, &user.Image, &user.Bio, &user.Email)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (b *Backend) IsRegistered(email string) (bool, error) {
	stmt, err := b.db.Prepare("SELECT COUNT(*) FROM users WHERE email = $1;")
	if err != nil {
		return false, err
	}

	var id int
	err = stmt.QueryRow(email).Scan(&id)
	if err != nil {
		return false, err
	}

	return id == 1, nil
}

func (b *Backend) FindByEmail(email string) (*types.User, error) {
	stmt, err := b.db.Prepare("SELECT id, display_name, username, image, bio, email FROM users WHERE email = $1;")
	if err != nil {
		return nil, err
	}

	user := &types.User{}
	err = stmt.QueryRow(email).Scan(&user.ID, &user.DisplayName, &user.Username, &user.Image, &user.Bio, &user.Email)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (b *Backend) CreateUser(email, displayName, bio, image, username string) (int, error) {
	stmt, err := b.db.Prepare("INSERT INTO users (display_name, username, email, bio, image) VALUES ($1, $2, $3, $4, $5) RETURNING id;")
	if err != nil {
		return 0, err
	}

	var id int
	err = stmt.QueryRow(displayName, strings.ToLower(username), email, bio, image).Scan(&id)
	if err != nil {
		return 0, err
	}

	return id, nil
}

func (b *Backend) CreateUserWithAppleLogin(email, displayName, bio, image, username, appleID string) (int, error) {
	ctx := context.Background()
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}

	res := tx.QueryRow(
		"INSERT INTO users (display_name, username, email, bio, image) VALUES ($1, $2, $3, $4, $5) RETURNING id;",
		displayName, strings.ToLower(username), email, bio, image,
	)

	var id int
	err = res.Scan(&id)
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}

	_, err = tx.ExecContext(
		ctx,
		"INSERT INTO apple_authentication (user_id, apple_user) VALUES ($1, $2);",
		id, appleID,
	)

	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}

	err = tx.Commit()
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}

	return id, nil
}

func (b *Backend) UpdateUser(id int, displayName, bio, image string) error {
	query := "UPDATE users SET display_name = $1, bio = $2, image = $3 WHERE id = $4;"

	stmt, err := b.db.Prepare(query)
	if err != nil {
		return err
	}

	_, err = stmt.Exec(displayName, bio, image, id)
	return err
}

func (b *Backend) GetProfileImage(id int) (string, error) {
	stmt, err := b.db.Prepare("SELECT image FROM users WHERE id = $1;")
	if err != nil {
		return "", err
	}

	r := stmt.QueryRow(id)

	var name string
	err = r.Scan(&name)
	if err != nil {
		return "", err
	}

	return name, err
}

func (b *Backend) LinkedAccounts(id int) ([]LinkedAccount, error) {
	stmt, err := b.db.Prepare("SELECT profile_id, username, provider FROM linked_accounts WHERE user_id = $1;")
	if err != nil {
		return nil, err
	}

	rows, err := stmt.Query(id)
	if err != nil {
		return nil, err
	}

	result := make([]LinkedAccount, 0)

	for rows.Next() {
		linked := LinkedAccount{}

		err := rows.Scan(&linked.ID, &linked.Username, &linked.Provider)
		if err != nil {
			return nil, err // @todo
		}

		result = append(result, linked)
	}

	return result, nil
}
