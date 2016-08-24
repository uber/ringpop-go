service RoleService {
    void SetRole(1: string role)
    list<string> GetMembers(1: string role)
}
